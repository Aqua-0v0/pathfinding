package zmap3base

import (
	"bufio"
	"encoding/binary"
	"io"
	"math"
)

type BinWriter struct {
	writer       *bufio.Writer
	littleEndian bool
	endianBuf    []byte
}

func NewBinWriter(file io.Writer, littleEndian bool) *BinWriter {
	return &BinWriter{
		writer:       bufio.NewWriter(file),
		littleEndian: littleEndian,
		endianBuf:    make([]byte, 8),
	}
}

func (w *BinWriter) Reset(file io.Writer) {
	w.writer.Reset(file)
}

func (w *BinWriter) WriteBool(v bool) {
	_ = w.writer.WriteByte(BoolToByte(v))
}

func (w *BinWriter) WriteUint8(v uint8) {
	_ = w.writer.WriteByte(v)
}

func (w *BinWriter) WriteUint16(v uint16) {
	_, _ = w.writer.Write(Uint16ToBytes(w.endianBuf, v, w.littleEndian))
}

func (w *BinWriter) WriteInt16(v int16) {
	_, _ = w.writer.Write(Int16ToBytes(w.endianBuf, v, w.littleEndian))
}

func (w *BinWriter) WriteUint32(v uint32) {
	_, _ = w.writer.Write(Uint32ToBytes(w.endianBuf, v, w.littleEndian))
}

func (w *BinWriter) WriteInt32(v int32) {
	_, _ = w.writer.Write(Int32ToBytes(w.endianBuf, v, w.littleEndian))
}

func (w *BinWriter) WriteUint64(v uint64) {
	_, _ = w.writer.Write(Uint64ToBytes(w.endianBuf, v, w.littleEndian))
}

func (w *BinWriter) WriteFloat64(v float64) {
	_, _ = w.writer.Write(Float64ToBytes(w.endianBuf, v, w.littleEndian))
}

func (w *BinWriter) Write(bts []byte) {
	_, _ = w.writer.Write(bts)
}

func (w *BinWriter) Flush() *BinWriter {
	_ = w.writer.Flush()
	return w
}

type BinReader struct {
	reader       io.Reader
	littleEndian bool
	endianBuf    []byte
}

func NewBinReader(file io.Reader, littleEndian bool) *BinReader {
	reader := baseBinReader(littleEndian)
	reader.reader = bufio.NewReader(file)
	return reader
}

func NewBinReaderNoBuffer(file io.Reader, littleEndian bool) *BinReader {
	reader := baseBinReader(littleEndian)
	reader.reader = file
	return reader
}

func baseBinReader(littleEndian bool) *BinReader {
	return &BinReader{
		littleEndian: littleEndian,
		endianBuf:    make([]byte, 8),
	}
}

func (r *BinReader) Reset(file io.Reader) {
	if raw, ok := r.reader.(*bufio.Reader); ok && raw != nil {
		raw.Reset(file)
	} else {
		r.reader = file
	}
}

func (r *BinReader) ByteOrder() binary.ByteOrder {
	if r.littleEndian {
		return binary.LittleEndian
	} else {
		return binary.BigEndian
	}
}

func (r *BinReader) Read(bts []byte) bool {
	l, err := io.ReadFull(r.reader, bts)
	if err != nil || l < 0 {
		return false
	}
	return true
}

func (r *BinReader) ReadN(p []byte) (n int) {
	n, _ = io.ReadFull(r.reader, p)
	return
}

func (r *BinReader) ReadBool() bool {
	_, err := io.ReadFull(r.reader, r.endianBuf[:1])
	if err != nil {
		panic(err)
	}
	return r.endianBuf[0] != 0
}

func (r *BinReader) ReadUint8() uint8 {
	_, err := io.ReadFull(r.reader, r.endianBuf[:1])
	if err != nil {
		panic(err)
	}
	return r.endianBuf[0]
}

func (r *BinReader) ReadInt8() int8 {
	return int8(r.ReadUint8())
}

func (r *BinReader) ReadUint16() uint16 {
	_, err := io.ReadFull(r.reader, r.endianBuf[:2])
	if err != nil {
		panic(err)
	}
	if r.littleEndian {
		return binary.LittleEndian.Uint16(r.endianBuf)
	} else {
		return binary.BigEndian.Uint16(r.endianBuf)
	}
}

func (r *BinReader) ReadInt16() int16 {
	return int16(r.ReadUint16())
}

func (r *BinReader) ReadUint32() uint32 {
	_, err := io.ReadFull(r.reader, r.endianBuf[:4])
	if err != nil {
		panic(err)
	}
	if r.littleEndian {
		return binary.LittleEndian.Uint32(r.endianBuf)
	} else {
		return binary.BigEndian.Uint32(r.endianBuf)
	}
}

func (r *BinReader) ReadInt32() int32 {
	return int32(r.ReadUint32())
}

func (r *BinReader) ReadUint64() uint64 {
	_, err := io.ReadFull(r.reader, r.endianBuf[:8])
	if err != nil {
		panic(err)
	}
	if r.littleEndian {
		return binary.LittleEndian.Uint64(r.endianBuf)
	} else {
		return binary.BigEndian.Uint64(r.endianBuf)
	}
}

func (r *BinReader) ReadFloat32() float32 {
	_, err := io.ReadFull(r.reader, r.endianBuf[:4])
	if err != nil {
		panic(err)
	}
	var u32 uint32
	if r.littleEndian {
		u32 = binary.LittleEndian.Uint32(r.endianBuf)
	} else {
		u32 = binary.BigEndian.Uint32(r.endianBuf)
	}
	return math.Float32frombits(u32)
}

func (r *BinReader) ReadFloat64() float64 {
	_, err := io.ReadFull(r.reader, r.endianBuf[:8])
	if err != nil {
		panic(err)
	}
	var u64 uint64
	if r.littleEndian {
		u64 = binary.LittleEndian.Uint64(r.endianBuf)
	} else {
		u64 = binary.BigEndian.Uint64(r.endianBuf)
	}
	return math.Float64frombits(u64)
}

func BoolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func Int16ToBytes(dst []byte, i16 int16, littleEndian bool) []byte {
	if littleEndian {
		binary.LittleEndian.PutUint16(dst, uint16(i16))
	} else {
		binary.BigEndian.PutUint16(dst, uint16(i16))
	}
	return dst[:2]
}

func Uint16ToBytes(dst []byte, u16 uint16, littleEndian bool) []byte {
	if littleEndian {
		binary.LittleEndian.PutUint16(dst, u16)
	} else {
		binary.BigEndian.PutUint16(dst, u16)
	}
	return dst[:2]
}

func Int32ToBytes(dst []byte, i32 int32, littleEndian bool) []byte {
	if littleEndian {
		binary.LittleEndian.PutUint32(dst, uint32(i32))
	} else {
		binary.BigEndian.PutUint32(dst, uint32(i32))
	}
	return dst[:4]
}

func Uint32ToBytes(dst []byte, u32 uint32, littleEndian bool) []byte {
	if littleEndian {
		binary.LittleEndian.PutUint32(dst, u32)
	} else {
		binary.BigEndian.PutUint32(dst, u32)
	}
	return dst[:4]
}

func Uint64ToBytes(dst []byte, u64 uint64, littleEndian bool) []byte {
	if littleEndian {
		binary.LittleEndian.PutUint64(dst, u64)
	} else {
		binary.BigEndian.PutUint64(dst, u64)
	}
	return dst[:8]
}

func Float64ToBytes(dst []byte, f64 float64, littleEndian bool) []byte {
	bb := math.Float64bits(f64)
	if littleEndian {
		binary.LittleEndian.PutUint64(dst, bb)
	} else {
		binary.BigEndian.PutUint64(dst, bb)
	}
	return dst[:8]
}
