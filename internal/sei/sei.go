// Package sei parses SEI metadata embedded in Tesla dashcam MP4 files.
// It is a pure Go port of sei_extractor.py from the dashcam-master reference.
package sei

import (
	"encoding/binary"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"

	"github.com/jacobalberty/dashmap/internal/pb"
)

// ExtractorVersion is bumped whenever the parsing algorithm changes in a way
// that would produce different track_point rows for the same file.  Ingest uses
// this to decide whether to reprocess a segment.
const ExtractorVersion = 1

// Result is returned by Extract.
type Result struct {
	HasSEI    bool
	FPS       float64
	DurationS float64
	Samples   []*pb.SeiMetadata // in file order
}

// Extract reads the MP4 at r, parses SEI metadata from all NALs, and returns
// the full sample list in file order.  fallbackFPS is used when the MP4 header
// cannot be parsed for fps.
func Extract(r io.ReadSeeker, fallbackFPS float64) (*Result, error) {
	fps, durationS, fpsOK := readFPSAndDuration(r)
	if !fpsOK {
		fps = fallbackFPS
	}

	offset, size, err := findMdat(r)
	if err != nil {
		return nil, fmt.Errorf("sei: find mdat: %w", err)
	}

	var samples []*pb.SeiMetadata
	if err := iterNALs(r, offset, size, func(nal []byte) {
		payload := extractProtoPayload(nal)
		if payload == nil {
			return
		}
		var m pb.SeiMetadata
		if err := proto.Unmarshal(payload, &m); err != nil {
			return
		}
		samples = append(samples, &m)
	}); err != nil {
		return nil, err
	}

	return &Result{
		HasSEI:    len(samples) > 0,
		FPS:       fps,
		DurationS: durationS,
		Samples:   samples,
	}, nil
}

// Probe returns true if the file contains at least one SEI user-data-unregistered
// NAL with a valid protobuf payload.  It returns early on the first match, so
// it is cheap to call on non-representative camera files.
func Probe(r io.ReadSeeker) (bool, error) {
	offset, size, err := findMdat(r)
	if err != nil {
		return false, fmt.Errorf("sei: probe find mdat: %w", err)
	}

	found := false
	if err := iterNALs(r, offset, size, func(nal []byte) {
		if found {
			return
		}
		payload := extractProtoPayload(nal)
		if payload == nil {
			return
		}
		var m pb.SeiMetadata
		if proto.Unmarshal(payload, &m) == nil {
			found = true
		}
	}); err != nil && !found {
		return false, err
	}
	return found, nil
}

// findMdat returns the byte offset and byte size of the mdat payload.
// size==0 means "extends to EOF" (consistent with the Python reference).
func findMdat(r io.ReadSeeker) (offset, size int64, err error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, 0, err
	}
	for {
		var hdr [8]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return 0, 0, fmt.Errorf("mdat atom not found")
		}
		size32 := binary.BigEndian.Uint32(hdr[:4])
		var typ [4]byte
		copy(typ[:], hdr[4:8])

		var atomSize int64
		headerSize := int64(8)
		switch size32 {
		case 1:
			var ext [8]byte
			if _, err := io.ReadFull(r, ext[:]); err != nil {
				return 0, 0, fmt.Errorf("truncated extended atom size")
			}
			atomSize = int64(binary.BigEndian.Uint64(ext[:])) //nolint:gosec // G115: file-size bounds checked by caller
			headerSize = 16
		case 0:
			atomSize = 0 // extends to EOF
		default:
			atomSize = int64(size32)
		}

		if typ == boxName("mdat") {
			payloadSize := int64(0)
			if atomSize != 0 {
				payloadSize = atomSize - headerSize
			}
			pos, err := r.Seek(0, io.SeekCurrent)
			if err != nil {
				return 0, 0, err
			}
			return pos, payloadSize, nil
		}

		if atomSize == 0 {
			return 0, 0, fmt.Errorf("mdat atom not found")
		}
		if atomSize < headerSize {
			return 0, 0, fmt.Errorf("invalid MP4 atom size %d", atomSize)
		}
		if _, err := r.Seek(atomSize-headerSize, io.SeekCurrent); err != nil {
			return 0, 0, err
		}
	}
}

// iterNALs walks the length-prefixed NALs in the mdat payload and calls fn for
// each NAL that is type 6 (SEI) subtype 5 (user-data-unregistered).
func iterNALs(r io.ReadSeeker, offset, size int64, fn func([]byte)) error {
	if _, err := r.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	var consumed int64
	for size == 0 || consumed < size {
		var lenBuf [4]byte
		if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
			break // EOF or short read — normal end of mdat
		}
		nalSize := int64(binary.BigEndian.Uint32(lenBuf[:]))
		consumed += 4

		if nalSize < 2 {
			if _, err := r.Seek(nalSize, io.SeekCurrent); err != nil {
				return err
			}
			consumed += nalSize
			continue
		}

		var first2 [2]byte
		if _, err := io.ReadFull(r, first2[:]); err != nil {
			break
		}
		if (first2[0]&0x1F) != 6 || first2[1] != 5 {
			// not SEI user-data-unregistered — skip
			if _, err := r.Seek(nalSize-2, io.SeekCurrent); err != nil {
				return err
			}
			consumed += nalSize
			continue
		}

		rest := make([]byte, nalSize-2)
		if _, err := io.ReadFull(r, rest); err != nil {
			break
		}
		nal := make([]byte, nalSize)
		nal[0] = first2[0]
		nal[1] = first2[1]
		copy(nal[2:], rest)
		consumed += nalSize
		fn(nal)
	}
	return nil
}

// extractProtoPayload extracts the protobuf payload from a SEI NAL unit.
// It ports the Python reference exactly: starting at index 3, skip 0x42 bytes,
// then require a 0x69 byte, then strip emulation-prevention bytes from the
// remainder (excluding the RBSP stop byte at the end).
func extractProtoPayload(nal []byte) []byte {
	if len(nal) < 2 {
		return nil
	}
	for i := 3; i < len(nal)-1; i++ {
		b := nal[i]
		if b == 0x42 {
			continue
		}
		if b == 0x69 {
			if i <= 2 {
				return nil
			}
			return stripEmulationPrevention(nal[i+1 : len(nal)-1])
		}
		break
	}
	return nil
}

// stripEmulationPrevention removes H.264 emulation-prevention bytes:
// any 0x03 byte that follows two consecutive 0x00 bytes is dropped.
func stripEmulationPrevention(data []byte) []byte {
	out := make([]byte, 0, len(data))
	zeros := 0
	for _, b := range data {
		if zeros >= 2 && b == 0x03 {
			zeros = 0
			continue
		}
		out = append(out, b)
		if b == 0x00 {
			zeros++
		} else {
			zeros = 0
		}
	}
	return out
}
