package sei

import (
	"encoding/binary"
	"fmt"
	"io"
)

// box holds the byte range of an MP4 box's payload (after the header).
type box struct {
	start int64
	end   int64
}

// findBox scans for a named 4-byte box type within [start, end) of r and
// returns the payload range.  It does not descend into containers.
func findBox(r io.ReadSeeker, start, end int64, name [4]byte) (box, error) {
	if _, err := r.Seek(start, io.SeekStart); err != nil {
		return box{}, err
	}
	pos := start
	for pos+8 <= end {
		var hdr [8]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return box{}, err
		}
		size32 := binary.BigEndian.Uint32(hdr[:4])
		var typ [4]byte
		copy(typ[:], hdr[4:8])

		var boxSize int64
		headerSize := int64(8)
		switch size32 {
		case 1: // extended 64-bit size
			var ext [8]byte
			if _, err := io.ReadFull(r, ext[:]); err != nil {
				return box{}, err
			}
			boxSize = int64(binary.BigEndian.Uint64(ext[:]))
			headerSize = 16
		case 0: // box extends to EOF
			boxSize = end - pos
		default:
			boxSize = int64(size32)
		}
		if boxSize < headerSize {
			return box{}, fmt.Errorf("invalid box size %d at offset %d", boxSize, pos)
		}
		payloadStart := pos + headerSize
		payloadEnd := pos + boxSize

		if typ == name {
			return box{start: payloadStart, end: payloadEnd}, nil
		}
		pos = payloadEnd
		if _, err := r.Seek(pos, io.SeekStart); err != nil {
			return box{}, err
		}
	}
	return box{}, fmt.Errorf("box %q not found", name)
}

func boxName(s string) [4]byte {
	var b [4]byte
	copy(b[:], s)
	return b
}

// readFPSAndDuration parses the first trak's mdhd and stts to compute the
// average fps and clip duration.  Returns ok=false on any parse error so the
// caller can fall back to the configured default.
func readFPSAndDuration(r io.ReadSeeker) (fps, durationS float64, ok bool) {
	size, err := seekerSize(r)
	if err != nil {
		return 0, 0, false
	}

	moov, err := findBox(r, 0, size, boxName("moov"))
	if err != nil {
		return 0, 0, false
	}
	trak, err := findBox(r, moov.start, moov.end, boxName("trak"))
	if err != nil {
		return 0, 0, false
	}
	mdia, err := findBox(r, trak.start, trak.end, boxName("mdia"))
	if err != nil {
		return 0, 0, false
	}

	// mdhd: version(1) + flags(3) + creation(4|8) + modification(4|8) + timescale(4) + duration(4|8)
	mdhd, err := findBox(r, mdia.start, mdia.end, boxName("mdhd"))
	if err != nil {
		return 0, 0, false
	}
	if _, err := r.Seek(mdhd.start, io.SeekStart); err != nil {
		return 0, 0, false
	}
	var versionByte [1]byte
	if _, err := io.ReadFull(r, versionByte[:]); err != nil {
		return 0, 0, false
	}
	version := versionByte[0]
	// skip flags(3)
	if _, err := r.Seek(3, io.SeekCurrent); err != nil {
		return 0, 0, false
	}
	var timescale uint32
	var mdhdDuration uint64
	if version == 1 {
		// creation_time(8) + modification_time(8) + timescale(4) + duration(8)
		var buf [28]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, 0, false
		}
		timescale = binary.BigEndian.Uint32(buf[16:20])
		mdhdDuration = binary.BigEndian.Uint64(buf[20:28])
	} else {
		// creation_time(4) + modification_time(4) + timescale(4) + duration(4)
		var buf [16]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, 0, false
		}
		timescale = binary.BigEndian.Uint32(buf[8:12])
		mdhdDuration = uint64(binary.BigEndian.Uint32(buf[12:16]))
	}
	if timescale == 0 {
		return 0, 0, false
	}

	// stts: version(1)+flags(3)+entry_count(4) then pairs of (sample_count(4), sample_delta(4))
	minf, err := findBox(r, mdia.start, mdia.end, boxName("minf"))
	if err != nil {
		return 0, 0, false
	}
	stbl, err := findBox(r, minf.start, minf.end, boxName("stbl"))
	if err != nil {
		return 0, 0, false
	}
	stts, err := findBox(r, stbl.start, stbl.end, boxName("stts"))
	if err != nil {
		return 0, 0, false
	}
	if _, err := r.Seek(stts.start+4, io.SeekStart); err != nil { // skip version+flags
		return 0, 0, false
	}
	var entryCount uint32
	if err := binary.Read(r, binary.BigEndian, &entryCount); err != nil {
		return 0, 0, false
	}
	var totalFrames, totalDeltas uint64
	for i := uint32(0); i < entryCount; i++ {
		var count, delta uint32
		if err := binary.Read(r, binary.BigEndian, &count); err != nil {
			return 0, 0, false
		}
		if err := binary.Read(r, binary.BigEndian, &delta); err != nil {
			return 0, 0, false
		}
		totalFrames += uint64(count)
		totalDeltas += uint64(count) * uint64(delta)
	}
	if totalDeltas == 0 {
		return 0, 0, false
	}

	fps = float64(totalFrames) * float64(timescale) / float64(totalDeltas)
	durationS = float64(mdhdDuration) / float64(timescale)
	return fps, durationS, true
}

// seekerSize seeks to the end and returns the file size, then seeks back.
func seekerSize(r io.ReadSeeker) (int64, error) {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	_, err = r.Seek(0, io.SeekStart)
	return size, err
}
