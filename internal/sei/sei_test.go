package sei

import (
	"bytes"
	"encoding/binary"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/jacobalberty/dashmap/internal/pb"
)

// buildMP4 builds a minimal MP4 byte slice containing an ftyp box followed by
// an mdat box whose payload is the provided raw mdat content.
func buildMP4(mdatPayload []byte) []byte {
	ftyp := []byte{
		0, 0, 0, 12, // size = 12
		'f', 't', 'y', 'p',
		'i', 's', 'o', 'm',
	}

	mdatSize := 8 + len(mdatPayload)
	mdat := make([]byte, mdatSize)
	binary.BigEndian.PutUint32(mdat[0:4], uint32(mdatSize))
	copy(mdat[4:8], "mdat")
	copy(mdat[8:], mdatPayload)

	return append(ftyp, mdat...)
}

// buildSEINAL builds a length-prefixed SEI NAL (type 6, subtype 5) wrapping
// a protobuf payload with the given marker prefix (0x42 run) and 0x69 marker.
func buildSEINAL(m *pb.SeiMetadata) []byte {
	protoBytes, err := proto.Marshal(m)
	if err != nil {
		panic(err)
	}

	// NAL body: [nal_header(1)] [subtype(1)] [payloadSize(1)] [0x42 0x42 0x42] [0x69] [proto...] [stop_bit(1)]
	// extractProtoPayload expects: scan from index 3, skip 0x42, require 0x69.
	// nal[0]=0x06 (type 6), nal[1]=0x05 (subtype 5), nal[2]=payloadSize, nal[3..]=marker+proto
	marker := []byte{0x42, 0x42, 0x42, 0x69}
	body := make([]byte, 0, 2+1+len(marker)+len(protoBytes)+1)
	body = append(body, 0x06, 0x05)              // header
	body = append(body, byte(len(protoBytes)+5)) // payload size byte (informational)
	body = append(body, marker...)
	body = append(body, protoBytes...)
	body = append(body, 0x80) // RBSP stop byte (stripped by extractProtoPayload)

	// Length-prefix (4-byte big-endian)
	nal := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(nal[0:4], uint32(len(body)))
	copy(nal[4:], body)
	return nal
}

// buildMoovBox builds a minimal moov→trak→mdia→mdhd+minf→stbl→stts box tree
// for the given timescale, frame count, and frame delta (stts single entry).
func buildMoovBox(timescale, frameCount, frameDelta uint32) []byte {
	// mdhd: version(1=0)+flags(3)+creation(4)+modification(4)+timescale(4)+duration(4) = 24 bytes payload
	duration := frameCount * frameDelta
	mdhd := makeBox("mdhd", []byte{
		0, 0, 0, 0, // version + flags
		0, 0, 0, 0, // creation_time
		0, 0, 0, 0, // modification_time
		byte(timescale >> 24), byte(timescale >> 16), byte(timescale >> 8), byte(timescale),
		byte(duration >> 24), byte(duration >> 16), byte(duration >> 8), byte(duration),
	})

	// stts: version(1)+flags(3)+entry_count(4)+count(4)+delta(4)
	stts := makeBox("stts", []byte{
		0, 0, 0, 0, // version+flags
		0, 0, 0, 1, // entry_count = 1
		byte(frameCount >> 24), byte(frameCount >> 16), byte(frameCount >> 8), byte(frameCount),
		byte(frameDelta >> 24), byte(frameDelta >> 16), byte(frameDelta >> 8), byte(frameDelta),
	})

	stbl := makeBox("stbl", stts)
	minf := makeBox("minf", stbl)
	mdia := makeBox("mdia", append(mdhd, minf...))
	trak := makeBox("trak", mdia)
	return makeBox("moov", trak)
}

func makeBox(name string, payload []byte) []byte {
	size := 8 + len(payload)
	b := make([]byte, size)
	binary.BigEndian.PutUint32(b[0:4], uint32(size))
	copy(b[4:8], name)
	copy(b[8:], payload)
	return b
}

func TestExtract_NoSEI(t *testing.T) {
	// An MP4 with an empty mdat contains no SEI; Extract should return HasSEI=false.
	mp4 := buildMP4(nil)
	r := bytes.NewReader(mp4)
	res, err := Extract(r, 36)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if res.HasSEI {
		t.Error("expected HasSEI=false for empty mdat")
	}
	if len(res.Samples) != 0 {
		t.Errorf("expected 0 samples, got %d", len(res.Samples))
	}
}

func TestExtract_WithSEI(t *testing.T) {
	want := &pb.SeiMetadata{
		Version:        1,
		FrameSeqNo:     93654,
		LatitudeDeg:    30.412841,
		LongitudeDeg:   -91.099508,
		VehicleSpeedMps: 11.16,
		GearState:      pb.SeiMetadata_GEAR_DRIVE,
	}
	nal := buildSEINAL(want)
	mp4 := buildMP4(nal)

	r := bytes.NewReader(mp4)
	res, err := Extract(r, 36)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !res.HasSEI {
		t.Fatal("expected HasSEI=true")
	}
	if len(res.Samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(res.Samples))
	}
	got := res.Samples[0]
	if got.FrameSeqNo != want.FrameSeqNo {
		t.Errorf("FrameSeqNo: got %d, want %d", got.FrameSeqNo, want.FrameSeqNo)
	}
	if got.GetLatitudeDeg() < 30.41 || got.GetLatitudeDeg() > 30.42 {
		t.Errorf("LatitudeDeg out of range: %f", got.GetLatitudeDeg())
	}
}

func TestExtract_MultipleSEI(t *testing.T) {
	var mdat []byte
	for i := range uint64(5) {
		mdat = append(mdat, buildSEINAL(&pb.SeiMetadata{FrameSeqNo: i})...)
	}
	r := bytes.NewReader(buildMP4(mdat))
	res, err := Extract(r, 36)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(res.Samples) != 5 {
		t.Fatalf("expected 5 samples, got %d", len(res.Samples))
	}
	for i, s := range res.Samples {
		if s.FrameSeqNo != uint64(i) {
			t.Errorf("sample %d: FrameSeqNo=%d, want %d", i, s.FrameSeqNo, i)
		}
	}
}

func TestProbe_True(t *testing.T) {
	mp4 := buildMP4(buildSEINAL(&pb.SeiMetadata{FrameSeqNo: 1}))
	r := bytes.NewReader(mp4)
	found, err := Probe(r)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !found {
		t.Error("expected Probe=true")
	}
}

func TestProbe_False(t *testing.T) {
	r := bytes.NewReader(buildMP4(nil))
	found, err := Probe(r)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if found {
		t.Error("expected Probe=false for empty mdat")
	}
}

func TestStripEmulationPrevention(t *testing.T) {
	// 00 00 03 xx → 00 00 xx  (0x03 after two 0x00s is stripped)
	in := []byte{0x00, 0x00, 0x03, 0x42, 0x00, 0x00, 0x03, 0x00}
	out := stripEmulationPrevention(in)
	want := []byte{0x00, 0x00, 0x42, 0x00, 0x00, 0x00}
	if !bytes.Equal(out, want) {
		t.Errorf("stripEmulationPrevention: got %x, want %x", out, want)
	}
}

func TestStripEmulationPrevention_NoOp(t *testing.T) {
	in := []byte{0x00, 0x01, 0x03, 0x00, 0x00, 0x04}
	out := stripEmulationPrevention(in)
	if !bytes.Equal(out, in) {
		t.Errorf("expected no change: got %x", out)
	}
}

func TestReadFPSAndDuration(t *testing.T) {
	// timescale=10000, frameCount=2154, frameDelta=278 → fps = 2154*10000/(2154*278) ≈ 35.97 ≈ 36
	moov := buildMoovBox(10000, 2154, 278)
	ftyp := makeBox("ftyp", []byte("isom"))
	mp4 := append(ftyp, moov...)
	mp4 = append(mp4, buildMP4(nil)[12:]...) // append an mdat

	r := bytes.NewReader(mp4)
	fps, dur, ok := readFPSAndDuration(r)
	if !ok {
		t.Fatal("readFPSAndDuration returned ok=false")
	}
	// fps should be ≈ 35.97
	if fps < 35 || fps > 37 {
		t.Errorf("fps out of expected range: %f", fps)
	}
	// duration = 2154*278/10000 = 59.8s
	if dur < 59 || dur > 61 {
		t.Errorf("durationS out of expected range: %f", dur)
	}
}

func TestReadFPSAndDuration_Fallback(t *testing.T) {
	// An MP4 with no moov box: readFPSAndDuration should return ok=false.
	mp4 := buildMP4(nil)
	r := bytes.NewReader(mp4)
	_, _, ok := readFPSAndDuration(r)
	if ok {
		t.Error("expected ok=false when no moov present")
	}
}

func TestExtract_FPSFromHeader(t *testing.T) {
	// Build an MP4 with a moov box (timescale=10000, 100 frames, delta=417 ≈ 24fps)
	// and an mdat with one SEI NAL.  Extract should use header fps, not fallback.
	moov := buildMoovBox(10000, 100, 417) // 10000/417 ≈ 23.98 fps
	ftyp := makeBox("ftyp", []byte("isom"))
	nal := buildSEINAL(&pb.SeiMetadata{FrameSeqNo: 0})
	mdat := makeBox("mdat", nal)
	mp4 := append(ftyp, moov...)
	mp4 = append(mp4, mdat...)

	r := bytes.NewReader(mp4)
	res, err := Extract(r, 99) // fallback=99 should NOT be used
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if res.FPS > 25 {
		t.Errorf("expected ~24fps from header, got %f (fallback 99 used?)", res.FPS)
	}
}
