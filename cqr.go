// [2022/05/10] [paepcke.de/internal/crq]
// [forked|inspired] by [github.com/skip2/go-qrcode] MIT
//
// WARNING:
// THIS IS AN HEAVYLY [MODIFIED|OPTIMIZED|MINIMAL] NOT API/RESULT COMPATIBLE FORK!
// DO NOT USE THIS FORK OUTSIDE THIS PACKAGE! ALL CREDITS GOES TO THE ORIGINAL AUTHOR(S)!
//
// PLEASE ALWAYS USE THE ORIGINAL SOURCE!
//
// ALL CREDIT GOES TO THE AUTHOR(S)!
//
// [github.com/skip2/go-qrcode] MIT LICENSE
//
// # Copyright (c) 2014 Tom Harwood
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
package cliqr

import (
	"bytes"
	"errors"
	"image/color"
)

//
// INTERNAL BACKEND
//

const (
	_blank      = " "
	_block      = "█"
	_blank_x2   = "  "
	_block_x2   = "██"
	_block_down = "▄"
	_block_up   = "▀"
	_lf         = "\n"

	b0 = false
	b1 = true

	penaltyWeight1 = 3
	penaltyWeight2 = 3
	penaltyWeight3 = 40
	penaltyWeight4 = 10

	formatInfoLengthBits  = 15
	versionInfoLengthBits = 18
)

const (
	dataModeNone dataMode = 1 << iota
	dataModeNumeric
	dataModeAlphanumeric
	dataModeByte
)

const (
	dataEncoderType1To9 dataEncoderType = iota
	dataEncoderType10To26
	dataEncoderType27To40
)

const (
	up direction = iota
	down
)

type (
	dataMode        uint8
	direction       uint8
	dataEncoderType uint8
	segment         struct {
		dataMode dataMode
		data     []byte
	}
	dataEncoder struct {
		minVersion                   int
		maxVersion                   int
		numericModeIndicator         *bitSet
		alphanumericModeIndicator    *bitSet
		byteModeIndicator            *bitSet
		numNumericCharCountBits      int
		numAlphanumericCharCountBits int
		numByteCharCountBits         int
		data                         []byte
		actual                       []segment
		optimised                    []segment
	}
	qrCode struct {
		Content         string
		Level           recoveryLevel
		VersionNumber   int
		ForegroundColor color.Color
		BackgroundColor color.Color
		DisableBorder   bool
		encoder         *dataEncoder
		version         qrCodeVersion
		data            *bitSet
		symbol          *symbol
		mask            int
	}
	regularSymbol struct {
		version qrCodeVersion
		mask    int
		data    *bitSet
		symbol  *symbol
		size    int
	}
	symbol struct {
		module        [][]bool
		isUsed        [][]bool
		size          int
		symbolSize    int
		quietZoneSize int
	}
)

func newDataEncoder(t dataEncoderType) *dataEncoder {
	d := &dataEncoder{}

	switch t {
	case dataEncoderType1To9:
		d = &dataEncoder{
			minVersion:                   1,
			maxVersion:                   9,
			numericModeIndicator:         bitsetnew(b0, b0, b0, b1),
			alphanumericModeIndicator:    bitsetnew(b0, b0, b1, b0),
			byteModeIndicator:            bitsetnew(b0, b1, b0, b0),
			numNumericCharCountBits:      10,
			numAlphanumericCharCountBits: 9,
			numByteCharCountBits:         8,
		}
	case dataEncoderType10To26:
		d = &dataEncoder{
			minVersion:                   10,
			maxVersion:                   26,
			numericModeIndicator:         bitsetnew(b0, b0, b0, b1),
			alphanumericModeIndicator:    bitsetnew(b0, b0, b1, b0),
			byteModeIndicator:            bitsetnew(b0, b1, b0, b0),
			numNumericCharCountBits:      12,
			numAlphanumericCharCountBits: 11,
			numByteCharCountBits:         16,
		}
	case dataEncoderType27To40:
		d = &dataEncoder{
			minVersion:                   27,
			maxVersion:                   40,
			numericModeIndicator:         bitsetnew(b0, b0, b0, b1),
			alphanumericModeIndicator:    bitsetnew(b0, b0, b1, b0),
			byteModeIndicator:            bitsetnew(b0, b1, b0, b0),
			numNumericCharCountBits:      14,
			numAlphanumericCharCountBits: 13,
			numByteCharCountBits:         16,
		}
	default:
		panic("Unknown dataEncoderType")
	}

	return d
}

func (d *dataEncoder) encode(data []byte) (*bitSet, error) {
	d.data = data
	d.actual = nil
	d.optimised = nil

	if len(data) == 0 {
		return nil, errors.New("no data to encode")
	}
	highestRequiredMode := d.classifyDataModes()
	err := d.optimiseDataModes()
	if err != nil {
		return nil, err
	}
	optimizedLength := 0
	for _, s := range d.optimised {
		length, err := d.encodedLength(s.dataMode, len(s.data))
		if err != nil {
			return nil, err
		}
		optimizedLength += length
	}
	singleByteSegmentLength, err := d.encodedLength(highestRequiredMode, len(d.data))
	if err != nil {
		return nil, err
	}
	if singleByteSegmentLength <= optimizedLength {
		d.optimised = []segment{{dataMode: highestRequiredMode, data: d.data}}
	}
	encoded := bitsetnew()
	for _, s := range d.optimised {
		d.encodeDataRaw(s.data, s.dataMode, encoded)
	}

	return encoded, nil
}

func (d *dataEncoder) classifyDataModes() dataMode {
	var start int
	mode := dataModeNone
	highestRequiredMode := mode

	for i, v := range d.data {
		newMode := dataModeNone
		switch {
		case v >= 0x30 && v <= 0x39:
			newMode = dataModeNumeric
		case v == 0x20 || v == 0x24 || v == 0x25 || v == 0x2a || v == 0x2b || v ==
			0x2d || v == 0x2e || v == 0x2f || v == 0x3a || (v >= 0x41 && v <= 0x5a):
			newMode = dataModeAlphanumeric
		default:
			newMode = dataModeByte
		}

		if newMode != mode {
			if i > 0 {
				d.actual = append(d.actual, segment{dataMode: mode, data: d.data[start:i]})

				start = i
			}

			mode = newMode
		}

		if newMode > highestRequiredMode {
			highestRequiredMode = newMode
		}
	}

	d.actual = append(d.actual, segment{dataMode: mode, data: d.data[start:len(d.data)]})

	return highestRequiredMode
}

func (d *dataEncoder) optimiseDataModes() error {
	for i := 0; i < len(d.actual); {
		mode := d.actual[i].dataMode
		numChars := len(d.actual[i].data)

		j := i + 1
		for j < len(d.actual) {
			nextNumChars := len(d.actual[j].data)
			nextMode := d.actual[j].dataMode

			if nextMode > mode {
				break
			}

			coalescedLength, err := d.encodedLength(mode, numChars+nextNumChars)
			if err != nil {
				return err
			}

			seperateLength1, err := d.encodedLength(mode, numChars)
			if err != nil {
				return err
			}

			seperateLength2, err := d.encodedLength(nextMode, nextNumChars)
			if err != nil {
				return err
			}

			if coalescedLength < seperateLength1+seperateLength2 {
				j++
				numChars += nextNumChars
			} else {
				break
			}
		}

		optimised := segment{
			dataMode: mode,
			data:     make([]byte, 0, numChars),
		}

		for k := i; k < j; k++ {
			optimised.data = append(optimised.data, d.actual[k].data...)
		}

		d.optimised = append(d.optimised, optimised)

		i = j
	}

	return nil
}

func (d *dataEncoder) encodeDataRaw(data []byte, dataMode dataMode, encoded *bitSet) {
	modeIndicator := d.modeIndicator(dataMode)
	charCountBits := d.charCountBits(dataMode)
	encoded.Append(modeIndicator)
	encoded.AppendUint32(uint32(len(data)), charCountBits)
	switch dataMode {
	case dataModeNumeric:
		for i := 0; i < len(data); i += 3 {
			charsRemaining := len(data) - i

			var value uint32
			bitsUsed := 1

			for j := 0; j < charsRemaining && j < 3; j++ {
				value *= 10
				value += uint32(data[i+j] - 0x30)
				bitsUsed += 3
			}
			encoded.AppendUint32(value, bitsUsed)
		}
	case dataModeAlphanumeric:
		for i := 0; i < len(data); i += 2 {
			charsRemaining := len(data) - i

			var value uint32
			for j := 0; j < charsRemaining && j < 2; j++ {
				value *= 45
				value += encodeAlphanumericCharacter(data[i+j])
			}

			bitsUsed := 6
			if charsRemaining > 1 {
				bitsUsed = 11
			}

			encoded.AppendUint32(value, bitsUsed)
		}
	case dataModeByte:
		for _, b := range data {
			encoded.AppendByte(b, 8)
		}
	}
}

func (d *dataEncoder) modeIndicator(dataMode dataMode) *bitSet {
	switch dataMode {
	case dataModeNumeric:
		return d.numericModeIndicator
	case dataModeAlphanumeric:
		return d.alphanumericModeIndicator
	case dataModeByte:
		return d.byteModeIndicator
	default:
		panic("Unknown data mode")
	}
}

func (d *dataEncoder) charCountBits(dataMode dataMode) int {
	switch dataMode {
	case dataModeNumeric:
		return d.numNumericCharCountBits
	case dataModeAlphanumeric:
		return d.numAlphanumericCharCountBits
	case dataModeByte:
		return d.numByteCharCountBits
	default:
		panic("Unknown data mode")
	}
}

func (d *dataEncoder) encodedLength(dataMode dataMode, n int) (int, error) {
	modeIndicator := d.modeIndicator(dataMode)
	charCountBits := d.charCountBits(dataMode)

	if modeIndicator == nil {
		return 0, errors.New("mode not supported")
	}

	maxLength := (1 << uint8(charCountBits)) - 1

	if n > maxLength {
		return 0, errors.New("length too long to be represented")
	}

	length := modeIndicator.Len() + charCountBits

	switch dataMode {
	case dataModeNumeric:
		length += 10 * (n / 3)

		if n%3 != 0 {
			length += 1 + 3*(n%3)
		}
	case dataModeAlphanumeric:
		length += 11 * (n / 2)
		length += 6 * (n % 2)
	case dataModeByte:
		length += 8 * n
	}

	return length, nil
}

func encodeAlphanumericCharacter(v byte) uint32 {
	c := uint32(v)

	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'A' && c <= 'Z':
		return c - 'A' + 10
	case c == ' ':
		return 36
	case c == '$':
		return 37
	case c == '%':
		return 38
	case c == '*':
		return 39
	case c == '+':
		return 40
	case c == '-':
		return 41
	case c == '.':
		return 42
	case c == '/':
		return 43
	case c == ':':
		return 44
	default:
		panic("encodeAlphanumericCharacter() ")
	}
}

func newQR(content string, level recoveryLevel) (*qrCode, error) {
	encoders := []dataEncoderType{
		dataEncoderType1To9, dataEncoderType10To26,
		dataEncoderType27To40,
	}

	var encoder *dataEncoder
	var encoded *bitSet
	var chosenVersion *qrCodeVersion
	var err error

	for _, t := range encoders {
		encoder = newDataEncoder(t)
		encoded, err = encoder.encode([]byte(content))

		if err != nil {
			continue
		}

		chosenVersion = chooseqrCodeVersion(level, encoder, encoded.Len())

		if chosenVersion != nil {
			break
		}
	}

	if err != nil {
		return nil, err
	} else if chosenVersion == nil {
		return nil, errors.New("content too long to encode")
	}

	q := &qrCode{
		Content: content,

		Level:         level,
		VersionNumber: chosenVersion.version,

		ForegroundColor: color.Black,
		BackgroundColor: color.White,

		encoder: encoder,
		data:    encoded,
		version: *chosenVersion,
	}

	return q, nil
}

func (q *qrCode) Bitmap() [][]bool {
	q.encode()
	return q.symbol.bitmap()
}

func (q *qrCode) encode() {
	numTerminatorBits := q.version.numTerminatorBitsRequired(q.data.Len())

	q.addTerminatorBits(numTerminatorBits)
	q.addPadding()

	encoded := q.encodeBlocks()

	const numMasks int = 8
	penalty := 0

	for mask := 0; mask < numMasks; mask++ {
		var s *symbol
		var err error

		s, err = buildRegularSymbol(q.version, mask, encoded, !q.DisableBorder)

		if err != nil {
			panic(err.Error())
		}

		numEmptyModules := s.numEmptyModules()
		if numEmptyModules != 0 {
			panic("bug: numEmptyModules")
		}

		p := s.penaltyScore()
		if q.symbol == nil || p < penalty {
			q.symbol = s
			q.mask = mask
			penalty = p
		}
	}
}

func (q *qrCode) addTerminatorBits(numTerminatorBits int) {
	q.data.AppendNumBools(numTerminatorBits, false)
}

func (q *qrCode) encodeBlocks() *bitSet {
	type dataBlock struct {
		data          *bitSet
		ecStartOffset int
	}

	block := make([]dataBlock, q.version.numBlocks())

	start := 0
	end := 0
	blockID := 0

	for _, b := range q.version.block {
		for j := 0; j < b.numBlocks; j++ {
			start = end
			end = start + b.numDataCodewords*8
			numErrorCodewords := b.numCodewords - b.numDataCodewords
			block[blockID].data = rEncode(q.data.Substr(start, end), numErrorCodewords)
			block[blockID].ecStartOffset = end - start

			blockID++
		}
	}
	result := bitsetnew()
	working := true
	for i := 0; working; i += 8 {
		working = false

		for j, b := range block {
			if i >= block[j].ecStartOffset {
				continue
			}

			result.Append(b.data.Substr(i, i+8))

			working = true
		}
	}
	working = true
	for i := 0; working; i += 8 {
		working = false

		for j, b := range block {
			offset := i + block[j].ecStartOffset
			if offset >= block[j].data.Len() {
				continue
			}

			result.Append(b.data.Substr(offset, offset+8))

			working = true
		}
	}
	result.AppendNumBools(q.version.numRemainderBits, false)

	return result
}

func (q *qrCode) addPadding() {
	numDataBits := q.version.numDataBits()

	if q.data.Len() == numDataBits {
		return
	}
	q.data.AppendNumBools(q.version.numBitsToPadToCodeword(q.data.Len()), false)
	padding := [2]*bitSet{
		bitsetnew(true, true, true, false, true, true, false, false),
		bitsetnew(false, false, false, true, false, false, false, true),
	}
	i := 0
	for numDataBits-q.data.Len() >= 8 {
		q.data.Append(padding[i])
		i = 1 - i // Alternate between 0 and 1.
	}
	if q.data.Len() != numDataBits {
		panic("BUG: got len = expected %d")
	}
}

var (
	alignmentPatternCenter = [][]int{
		{}, // Version 0 doesn't exist.
		{}, // Version 1 doesn't use alignment patterns.
		{6, 18},
		{6, 22},
		{6, 26},
		{6, 30},
		{6, 34},
		{6, 22, 38},
		{6, 24, 42},
		{6, 26, 46},
		{6, 28, 50},
		{6, 30, 54},
		{6, 32, 58},
		{6, 34, 62},
		{6, 26, 46, 66},
		{6, 26, 48, 70},
		{6, 26, 50, 74},
		{6, 30, 54, 78},
		{6, 30, 56, 82},
		{6, 30, 58, 86},
		{6, 34, 62, 90},
		{6, 28, 50, 72, 94},
		{6, 26, 50, 74, 98},
		{6, 30, 54, 78, 102},
		{6, 28, 54, 80, 106},
		{6, 32, 58, 84, 110},
		{6, 30, 58, 86, 114},
		{6, 34, 62, 90, 118},
		{6, 26, 50, 74, 98, 122},
		{6, 30, 54, 78, 102, 126},
		{6, 26, 52, 78, 104, 130},
		{6, 30, 56, 82, 108, 134},
		{6, 34, 60, 86, 112, 138},
		{6, 30, 58, 86, 114, 142},
		{6, 34, 62, 90, 118, 146},
		{6, 30, 54, 78, 102, 126, 150},
		{6, 24, 50, 76, 102, 128, 154},
		{6, 28, 54, 80, 106, 132, 158},
		{6, 32, 58, 84, 110, 136, 162},
		{6, 26, 54, 82, 110, 138, 166},
		{6, 30, 58, 86, 114, 142, 170},
	}

	finderPattern = [][]bool{
		{b1, b1, b1, b1, b1, b1, b1},
		{b1, b0, b0, b0, b0, b0, b1},
		{b1, b0, b1, b1, b1, b0, b1},
		{b1, b0, b1, b1, b1, b0, b1},
		{b1, b0, b1, b1, b1, b0, b1},
		{b1, b0, b0, b0, b0, b0, b1},
		{b1, b1, b1, b1, b1, b1, b1},
	}

	finderPatternSize = 7

	finderPatternHorizontalBorder = [][]bool{
		{b0, b0, b0, b0, b0, b0, b0, b0},
	}

	finderPatternVerticalBorder = [][]bool{
		{b0},
		{b0},
		{b0},
		{b0},
		{b0},
		{b0},
		{b0},
		{b0},
	}

	alignmentPattern = [][]bool{
		{b1, b1, b1, b1, b1},
		{b1, b0, b0, b0, b1},
		{b1, b0, b1, b0, b1},
		{b1, b0, b0, b0, b1},
		{b1, b1, b1, b1, b1},
	}
)

func buildRegularSymbol(version qrCodeVersion, mask int,
	data *bitSet, includeQuietZone bool,
) (*symbol, error) {
	quietZoneSize := 0
	if includeQuietZone {
		quietZoneSize = version.quietZoneSize()
	}
	m := &regularSymbol{
		version: version,
		mask:    mask,
		data:    data,

		symbol: newSymbol(version.symbolSize(), quietZoneSize),
		size:   version.symbolSize(),
	}
	m.addFinderPatterns()
	m.addAlignmentPatterns()
	m.addTimingPatterns()
	m.addFormatInfo()
	m.addVersionInfo()

	ok, err := m.addData()
	if !ok {
		return nil, err
	}
	return m.symbol, nil
}

func (m *regularSymbol) addFinderPatterns() {
	fpSize := finderPatternSize
	fp := finderPattern
	fpHBorder := finderPatternHorizontalBorder
	fpVBorder := finderPatternVerticalBorder
	m.symbol.set2dPattern(0, 0, fp)
	m.symbol.set2dPattern(0, fpSize, fpHBorder)
	m.symbol.set2dPattern(fpSize, 0, fpVBorder)
	m.symbol.set2dPattern(m.size-fpSize, 0, fp)
	m.symbol.set2dPattern(m.size-fpSize-1, fpSize, fpHBorder)
	m.symbol.set2dPattern(m.size-fpSize-1, 0, fpVBorder)
	m.symbol.set2dPattern(0, m.size-fpSize, fp)
	m.symbol.set2dPattern(0, m.size-fpSize-1, fpHBorder)
	m.symbol.set2dPattern(fpSize, m.size-fpSize-1, fpVBorder)
}

func (m *regularSymbol) addAlignmentPatterns() {
	for _, x := range alignmentPatternCenter[m.version.version] {
		for _, y := range alignmentPatternCenter[m.version.version] {
			if !m.symbol.empty(x, y) {
				continue
			}

			m.symbol.set2dPattern(x-2, y-2, alignmentPattern)
		}
	}
}

func (m *regularSymbol) addTimingPatterns() {
	value := true

	for i := finderPatternSize + 1; i < m.size-finderPatternSize; i++ {
		m.symbol.set(i, finderPatternSize-1, value)
		m.symbol.set(finderPatternSize-1, i, value)

		value = !value
	}
}

func (m *regularSymbol) addFormatInfo() {
	fpSize := finderPatternSize
	l := formatInfoLengthBits - 1
	f := m.version.formatInfo(m.mask)
	for i := 0; i <= 7; i++ {
		m.symbol.set(m.size-i-1, fpSize+1, f.At(l-i))
	}
	for i := 0; i <= 5; i++ {
		m.symbol.set(fpSize+1, i, f.At(l-i))
	}
	m.symbol.set(fpSize+1, fpSize, f.At(l-6))
	m.symbol.set(fpSize+1, fpSize+1, f.At(l-7))
	m.symbol.set(fpSize, fpSize+1, f.At(l-8))
	for i := 9; i <= 14; i++ {
		m.symbol.set(14-i, fpSize+1, f.At(l-i))
	}
	for i := 8; i <= 14; i++ {
		m.symbol.set(fpSize+1, m.size-fpSize+i-8, f.At(l-i))
	}
	m.symbol.set(fpSize+1, m.size-fpSize-1, true)
}

func (m *regularSymbol) addVersionInfo() {
	fpSize := finderPatternSize
	v := m.version.versionInfo()
	l := versionInfoLengthBits - 1
	if v == nil {
		return
	}
	for i := 0; i < v.Len(); i++ {
		m.symbol.set(i/3, m.size-fpSize-4+i%3, v.At(l-i))
		m.symbol.set(m.size-fpSize-4+i%3, i/3, v.At(l-i))
	}
}

func (m *regularSymbol) addData() (bool, error) {
	xOffset := 1
	dir := up

	x := m.size - 2
	y := m.size - 1

	for i := 0; i < m.data.Len(); i++ {
		var mask bool
		switch m.mask {
		case 0:
			mask = (y+x+xOffset)%2 == 0
		case 1:
			mask = y%2 == 0
		case 2:
			mask = (x+xOffset)%3 == 0
		case 3:
			mask = (y+x+xOffset)%3 == 0
		case 4:
			mask = (y/2+(x+xOffset)/3)%2 == 0
		case 5:
			mask = (y*(x+xOffset))%2+(y*(x+xOffset))%3 == 0
		case 6:
			mask = ((y*(x+xOffset))%2+((y*(x+xOffset))%3))%2 == 0
		case 7:
			mask = ((y+x+xOffset)%2+((y*(x+xOffset))%3))%2 == 0
		}

		// != is equivalent to XOR.
		m.symbol.set(x+xOffset, y, mask != m.data.At(i))

		if i == m.data.Len()-1 {
			break
		}

		// Find next free bit in the symbol.
		for {
			if xOffset == 1 {
				xOffset = 0
			} else {
				xOffset = 1

				if dir == up {
					if y > 0 {
						y--
					} else {
						dir = down
						x -= 2
					}
				} else {
					if y < m.size-1 {
						y++
					} else {
						dir = up
						x -= 2
					}
				}
			}

			// Skip over the vertical timing pattern entirely.
			if x == 5 {
				x--
			}

			if m.symbol.empty(x+xOffset, y) {
				break
			}
		}
	}

	return true, nil
}

func newSymbol(size, quietZoneSize int) *symbol {
	var m symbol

	m.module = make([][]bool, size+2*quietZoneSize)
	m.isUsed = make([][]bool, size+2*quietZoneSize)

	for i := range m.module {
		m.module[i] = make([]bool, size+2*quietZoneSize)
		m.isUsed[i] = make([]bool, size+2*quietZoneSize)
	}

	m.size = size + 2*quietZoneSize
	m.symbolSize = size
	m.quietZoneSize = quietZoneSize

	return &m
}

func (m *symbol) get(x, y int) (v bool) {
	v = m.module[y+m.quietZoneSize][x+m.quietZoneSize]
	return v
}

func (m *symbol) empty(x, y int) bool {
	return !m.isUsed[y+m.quietZoneSize][x+m.quietZoneSize]
}

func (m *symbol) numEmptyModules() int {
	var count int
	for y := 0; y < m.symbolSize; y++ {
		for x := 0; x < m.symbolSize; x++ {
			if !m.isUsed[y+m.quietZoneSize][x+m.quietZoneSize] {
				count++
			}
		}
	}

	return count
}

func (m *symbol) set(x, y int, v bool) {
	m.module[y+m.quietZoneSize][x+m.quietZoneSize] = v
	m.isUsed[y+m.quietZoneSize][x+m.quietZoneSize] = true
}

func (m *symbol) set2dPattern(x, y int, v [][]bool) {
	for j, row := range v {
		for i, value := range row {
			m.set(x+i, y+j, value)
		}
	}
}

func (m *symbol) bitmap() [][]bool {
	module := make([][]bool, len(m.module))

	for i := range m.module {
		module[i] = m.module[i][:]
	}

	return module
}

func (m *symbol) penaltyScore() int {
	return m.penalty1() + m.penalty2() + m.penalty3() + m.penalty4()
}

func (m *symbol) penalty1() int {
	penalty := 0

	for x := 0; x < m.symbolSize; x++ {
		lastValue := m.get(x, 0)
		count := 1

		for y := 1; y < m.symbolSize; y++ {
			v := m.get(x, y)

			if v != lastValue {
				count = 1
				lastValue = v
			} else {
				count++
				if count == 6 {
					penalty += penaltyWeight1 + 1
				} else if count > 6 {
					penalty++
				}
			}
		}
	}

	for y := 0; y < m.symbolSize; y++ {
		lastValue := m.get(0, y)
		count := 1

		for x := 1; x < m.symbolSize; x++ {
			v := m.get(x, y)

			if v != lastValue {
				count = 1
				lastValue = v
			} else {
				count++
				if count == 6 {
					penalty += penaltyWeight1 + 1
				} else if count > 6 {
					penalty++
				}
			}
		}
	}

	return penalty
}

func (m *symbol) penalty2() int {
	penalty := 0

	for y := 1; y < m.symbolSize; y++ {
		for x := 1; x < m.symbolSize; x++ {
			topLeft := m.get(x-1, y-1)
			above := m.get(x, y-1)
			left := m.get(x-1, y)
			current := m.get(x, y)

			if current == left && current == above && current == topLeft {
				penalty++
			}
		}
	}

	return penalty * penaltyWeight2
}

func (m *symbol) penalty3() int {
	penalty := 0

	for y := 0; y < m.symbolSize; y++ {
		var bitBuffer int16 = 0x00

		for x := 0; x < m.symbolSize; x++ {
			bitBuffer <<= 1
			if v := m.get(x, y); v {
				bitBuffer |= 1
			}

			switch bitBuffer & 0x7ff {
			case 0x05d, 0x5d0:
				penalty += penaltyWeight3
				bitBuffer = 0xFF
			default:
				if x == m.symbolSize-1 && (bitBuffer&0x7f) == 0x5d {
					penalty += penaltyWeight3
					bitBuffer = 0xFF
				}
			}
		}
	}

	for x := 0; x < m.symbolSize; x++ {
		var bitBuffer int16 = 0x00

		for y := 0; y < m.symbolSize; y++ {
			bitBuffer <<= 1
			if v := m.get(x, y); v {
				bitBuffer |= 1
			}

			switch bitBuffer & 0x7ff {
			case 0x05d, 0x5d0:
				penalty += penaltyWeight3
				bitBuffer = 0xFF
			default:
				if y == m.symbolSize-1 && (bitBuffer&0x7f) == 0x5d {
					penalty += penaltyWeight3
					bitBuffer = 0xFF
				}
			}
		}
	}

	return penalty
}

// penalty4 returns the penalty score...
func (m *symbol) penalty4() int {
	numModules := m.symbolSize * m.symbolSize
	numDarkModules := 0

	for x := 0; x < m.symbolSize; x++ {
		for y := 0; y < m.symbolSize; y++ {
			if v := m.get(x, y); v {
				numDarkModules++
			}
		}
	}

	numDarkModuleDeviation := numModules/2 - numDarkModules
	if numDarkModuleDeviation < 0 {
		numDarkModuleDeviation *= -1
	}

	return penaltyWeight4 * (numDarkModuleDeviation / (numModules / 20))
}

type recoveryLevel int

const (
	low recoveryLevel = iota
	medium
	high
	highest
)

type qrCodeVersion struct {
	version          int
	level            recoveryLevel
	dataEncoderType  dataEncoderType
	block            []block
	numRemainderBits int
}

type block struct {
	numBlocks        int
	numCodewords     int
	numDataCodewords int
}

var versions = []qrCodeVersion{
	{
		1,
		low,
		dataEncoderType1To9,
		[]block{
			{
				1,
				26,
				19,
			},
		},
		0,
	},
	{
		1,
		medium,
		dataEncoderType1To9,
		[]block{
			{
				1,
				26,
				16,
			},
		},
		0,
	},
	{
		1,
		high,
		dataEncoderType1To9,
		[]block{
			{
				1,
				26,
				13,
			},
		},
		0,
	},
	{
		1,
		highest,
		dataEncoderType1To9,
		[]block{
			{
				1,
				26,
				9,
			},
		},
		0,
	},
	{
		2,
		low,
		dataEncoderType1To9,
		[]block{
			{
				1,
				44,
				34,
			},
		},
		7,
	},
	{
		2,
		medium,
		dataEncoderType1To9,
		[]block{
			{
				1,
				44,
				28,
			},
		},
		7,
	},
	{
		2,
		high,
		dataEncoderType1To9,
		[]block{
			{
				1,
				44,
				22,
			},
		},
		7,
	},
	{
		2,
		highest,
		dataEncoderType1To9,
		[]block{
			{
				1,
				44,
				16,
			},
		},
		7,
	},
	{
		3,
		low,
		dataEncoderType1To9,
		[]block{
			{
				1,
				70,
				55,
			},
		},
		7,
	},
	{
		3,
		medium,
		dataEncoderType1To9,
		[]block{
			{
				1,
				70,
				44,
			},
		},
		7,
	},
	{
		3,
		high,
		dataEncoderType1To9,
		[]block{
			{
				2,
				35,
				17,
			},
		},
		7,
	},
	{
		3,
		highest,
		dataEncoderType1To9,
		[]block{
			{
				2,
				35,
				13,
			},
		},
		7,
	},
	{
		4,
		low,
		dataEncoderType1To9,
		[]block{
			{
				1,
				100,
				80,
			},
		},
		7,
	},
	{
		4,
		medium,
		dataEncoderType1To9,
		[]block{
			{
				2,
				50,
				32,
			},
		},
		7,
	},
	{
		4,
		high,
		dataEncoderType1To9,
		[]block{
			{
				2,
				50,
				24,
			},
		},
		7,
	},
	{
		4,
		highest,
		dataEncoderType1To9,
		[]block{
			{
				4,
				25,
				9,
			},
		},
		7,
	},
	{
		5,
		low,
		dataEncoderType1To9,
		[]block{
			{
				1,
				134,
				108,
			},
		},
		7,
	},
	{
		5,
		medium,
		dataEncoderType1To9,
		[]block{
			{
				2,
				67,
				43,
			},
		},
		7,
	},
	{
		5,
		high,
		dataEncoderType1To9,
		[]block{
			{
				2,
				33,
				15,
			},
			{
				2,
				34,
				16,
			},
		},
		7,
	},
	{
		5,
		highest,
		dataEncoderType1To9,
		[]block{
			{
				2,
				33,
				11,
			},
			{
				2,
				34,
				12,
			},
		},
		7,
	},
	{
		6,
		low,
		dataEncoderType1To9,
		[]block{
			{
				2,
				86,
				68,
			},
		},
		7,
	},
	{
		6,
		medium,
		dataEncoderType1To9,
		[]block{
			{
				4,
				43,
				27,
			},
		},
		7,
	},
	{
		6,
		high,
		dataEncoderType1To9,
		[]block{
			{
				4,
				43,
				19,
			},
		},
		7,
	},
	{
		6,
		highest,
		dataEncoderType1To9,
		[]block{
			{
				4,
				43,
				15,
			},
		},
		7,
	},
	{
		7,
		low,
		dataEncoderType1To9,
		[]block{
			{
				2,
				98,
				78,
			},
		},
		0,
	},
	{
		7,
		medium,
		dataEncoderType1To9,
		[]block{
			{
				4,
				49,
				31,
			},
		},
		0,
	},
	{
		7,
		high,
		dataEncoderType1To9,
		[]block{
			{
				2,
				32,
				14,
			},
			{
				4,
				33,
				15,
			},
		},
		0,
	},
	{
		7,
		highest,
		dataEncoderType1To9,
		[]block{
			{
				4,
				39,
				13,
			},
			{
				1,
				40,
				14,
			},
		},
		0,
	},
	{
		8,
		low,
		dataEncoderType1To9,
		[]block{
			{
				2,
				121,
				97,
			},
		},
		0,
	},
	{
		8,
		medium,
		dataEncoderType1To9,
		[]block{
			{
				2,
				60,
				38,
			},
			{
				2,
				61,
				39,
			},
		},
		0,
	},
	{
		8,
		high,
		dataEncoderType1To9,
		[]block{
			{
				4,
				40,
				18,
			},
			{
				2,
				41,
				19,
			},
		},
		0,
	},
	{
		8,
		highest,
		dataEncoderType1To9,
		[]block{
			{
				4,
				40,
				14,
			},
			{
				2,
				41,
				15,
			},
		},
		0,
	},
	{
		9,
		low,
		dataEncoderType1To9,
		[]block{
			{
				2,
				146,
				116,
			},
		},
		0,
	},
	{
		9,
		medium,
		dataEncoderType1To9,
		[]block{
			{
				3,
				58,
				36,
			},
			{
				2,
				59,
				37,
			},
		},
		0,
	},
	{
		9,
		high,
		dataEncoderType1To9,
		[]block{
			{
				4,
				36,
				16,
			},
			{
				4,
				37,
				17,
			},
		},
		0,
	},
	{
		9,
		highest,
		dataEncoderType1To9,
		[]block{
			{
				4,
				36,
				12,
			},
			{
				4,
				37,
				13,
			},
		},
		0,
	},
	{
		10,
		low,
		dataEncoderType10To26,
		[]block{
			{
				2,
				86,
				68,
			},
			{
				2,
				87,
				69,
			},
		},
		0,
	},
	{
		10,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				4,
				69,
				43,
			},
			{
				1,
				70,
				44,
			},
		},
		0,
	},
	{
		10,
		high,
		dataEncoderType10To26,
		[]block{
			{
				6,
				43,
				19,
			},
			{
				2,
				44,
				20,
			},
		},
		0,
	},
	{
		10,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				6,
				43,
				15,
			},
			{
				2,
				44,
				16,
			},
		},
		0,
	},
	{
		11,
		low,
		dataEncoderType10To26,
		[]block{
			{
				4,
				101,
				81,
			},
		},
		0,
	},
	{
		11,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				1,
				80,
				50,
			},
			{
				4,
				81,
				51,
			},
		},
		0,
	},
	{
		11,
		high,
		dataEncoderType10To26,
		[]block{
			{
				4,
				50,
				22,
			},
			{
				4,
				51,
				23,
			},
		},
		0,
	},
	{
		11,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				3,
				36,
				12,
			},
			{
				8,
				37,
				13,
			},
		},
		0,
	},
	{
		12,
		low,
		dataEncoderType10To26,
		[]block{
			{
				2,
				116,
				92,
			},
			{
				2,
				117,
				93,
			},
		},
		0,
	},
	{
		12,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				6,
				58,
				36,
			},
			{
				2,
				59,
				37,
			},
		},
		0,
	},
	{
		12,
		high,
		dataEncoderType10To26,
		[]block{
			{
				4,
				46,
				20,
			},
			{
				6,
				47,
				21,
			},
		},
		0,
	},
	{
		12,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				7,
				42,
				14,
			},
			{
				4,
				43,
				15,
			},
		},
		0,
	},
	{
		13,
		low,
		dataEncoderType10To26,
		[]block{
			{
				4,
				133,
				107,
			},
		},
		0,
	},
	{
		13,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				8,
				59,
				37,
			},
			{
				1,
				60,
				38,
			},
		},
		0,
	},
	{
		13,
		high,
		dataEncoderType10To26,
		[]block{
			{
				8,
				44,
				20,
			},
			{
				4,
				45,
				21,
			},
		},
		0,
	},
	{
		13,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				12,
				33,
				11,
			},
			{
				4,
				34,
				12,
			},
		},
		0,
	},
	{
		14,
		low,
		dataEncoderType10To26,
		[]block{
			{
				3,
				145,
				115,
			},
			{
				1,
				146,
				116,
			},
		},
		3,
	},
	{
		14,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				4,
				64,
				40,
			},
			{
				5,
				65,
				41,
			},
		},
		3,
	},
	{
		14,
		high,
		dataEncoderType10To26,
		[]block{
			{
				11,
				36,
				16,
			},
			{
				5,
				37,
				17,
			},
		},
		3,
	},
	{
		14,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				11,
				36,
				12,
			},
			{
				5,
				37,
				13,
			},
		},
		3,
	},
	{
		15,
		low,
		dataEncoderType10To26,
		[]block{
			{
				5,
				109,
				87,
			},
			{
				1,
				110,
				88,
			},
		},
		3,
	},
	{
		15,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				5,
				65,
				41,
			},
			{
				5,
				66,
				42,
			},
		},
		3,
	},
	{
		15,
		high,
		dataEncoderType10To26,
		[]block{
			{
				5,
				54,
				24,
			},
			{
				7,
				55,
				25,
			},
		},
		3,
	},
	{
		15,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				11,
				36,
				12,
			},
			{
				7,
				37,
				13,
			},
		},
		3,
	},
	{
		16,
		low,
		dataEncoderType10To26,
		[]block{
			{
				5,
				122,
				98,
			},
			{
				1,
				123,
				99,
			},
		},
		3,
	},
	{
		16,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				7,
				73,
				45,
			},
			{
				3,
				74,
				46,
			},
		},
		3,
	},
	{
		16,
		high,
		dataEncoderType10To26,
		[]block{
			{
				15,
				43,
				19,
			},
			{
				2,
				44,
				20,
			},
		},
		3,
	},
	{
		16,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				3,
				45,
				15,
			},
			{
				13,
				46,
				16,
			},
		},
		3,
	},
	{
		17,
		low,
		dataEncoderType10To26,
		[]block{
			{
				1,
				135,
				107,
			},
			{
				5,
				136,
				108,
			},
		},
		3,
	},
	{
		17,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				10,
				74,
				46,
			},
			{
				1,
				75,
				47,
			},
		},
		3,
	},
	{
		17,
		high,
		dataEncoderType10To26,
		[]block{
			{
				1,
				50,
				22,
			},
			{
				15,
				51,
				23,
			},
		},
		3,
	},
	{
		17,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				2,
				42,
				14,
			},
			{
				17,
				43,
				15,
			},
		},
		3,
	},
	{
		18,
		low,
		dataEncoderType10To26,
		[]block{
			{
				5,
				150,
				120,
			},
			{
				1,
				151,
				121,
			},
		},
		3,
	},
	{
		18,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				9,
				69,
				43,
			},
			{
				4,
				70,
				44,
			},
		},
		3,
	},
	{
		18,
		high,
		dataEncoderType10To26,
		[]block{
			{
				17,
				50,
				22,
			},
			{
				1,
				51,
				23,
			},
		},
		3,
	},
	{
		18,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				2,
				42,
				14,
			},
			{
				19,
				43,
				15,
			},
		},
		3,
	},
	{
		19,
		low,
		dataEncoderType10To26,
		[]block{
			{
				3,
				141,
				113,
			},
			{
				4,
				142,
				114,
			},
		},
		3,
	},
	{
		19,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				3,
				70,
				44,
			},
			{
				11,
				71,
				45,
			},
		},
		3,
	},
	{
		19,
		high,
		dataEncoderType10To26,
		[]block{
			{
				17,
				47,
				21,
			},
			{
				4,
				48,
				22,
			},
		},
		3,
	},
	{
		19,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				9,
				39,
				13,
			},
			{
				16,
				40,
				14,
			},
		},
		3,
	},
	{
		20,
		low,
		dataEncoderType10To26,
		[]block{
			{
				3,
				135,
				107,
			},
			{
				5,
				136,
				108,
			},
		},
		3,
	},
	{
		20,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				3,
				67,
				41,
			},
			{
				13,
				68,
				42,
			},
		},
		3,
	},
	{
		20,
		high,
		dataEncoderType10To26,
		[]block{
			{
				15,
				54,
				24,
			},
			{
				5,
				55,
				25,
			},
		},
		3,
	},
	{
		20,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				15,
				43,
				15,
			},
			{
				10,
				44,
				16,
			},
		},
		3,
	},
	{
		21,
		low,
		dataEncoderType10To26,
		[]block{
			{
				4,
				144,
				116,
			},
			{
				4,
				145,
				117,
			},
		},
		4,
	},
	{
		21,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				17,
				68,
				42,
			},
		},
		4,
	},
	{
		21,
		high,
		dataEncoderType10To26,
		[]block{
			{
				17,
				50,
				22,
			},
			{
				6,
				51,
				23,
			},
		},
		4,
	},
	{
		21,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				19,
				46,
				16,
			},
			{
				6,
				47,
				17,
			},
		},
		4,
	},
	{
		22,
		low,
		dataEncoderType10To26,
		[]block{
			{
				2,
				139,
				111,
			},
			{
				7,
				140,
				112,
			},
		},
		4,
	},
	{
		22,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				17,
				74,
				46,
			},
		},
		4,
	},
	{
		22,
		high,
		dataEncoderType10To26,
		[]block{
			{
				7,
				54,
				24,
			},
			{
				16,
				55,
				25,
			},
		},
		4,
	},
	{
		22,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				34,
				37,
				13,
			},
		},
		4,
	},
	{
		23,
		low,
		dataEncoderType10To26,
		[]block{
			{
				4,
				151,
				121,
			},
			{
				5,
				152,
				122,
			},
		},
		4,
	},
	{
		23,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				4,
				75,
				47,
			},
			{
				14,
				76,
				48,
			},
		},
		4,
	},
	{
		23,
		high,
		dataEncoderType10To26,
		[]block{
			{
				11,
				54,
				24,
			},
			{
				14,
				55,
				25,
			},
		},
		4,
	},
	{
		23,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				16,
				45,
				15,
			},
			{
				14,
				46,
				16,
			},
		},
		4,
	},
	{
		24,
		low,
		dataEncoderType10To26,
		[]block{
			{
				6,
				147,
				117,
			},
			{
				4,
				148,
				118,
			},
		},
		4,
	},
	{
		24,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				6,
				73,
				45,
			},
			{
				14,
				74,
				46,
			},
		},
		4,
	},
	{
		24,
		high,
		dataEncoderType10To26,
		[]block{
			{
				11,
				54,
				24,
			},
			{
				16,
				55,
				25,
			},
		},
		4,
	},
	{
		24,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				30,
				46,
				16,
			},
			{
				2,
				47,
				17,
			},
		},
		4,
	},
	{
		25,
		low,
		dataEncoderType10To26,
		[]block{
			{
				8,
				132,
				106,
			},
			{
				4,
				133,
				107,
			},
		},
		4,
	},
	{
		25,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				8,
				75,
				47,
			},
			{
				13,
				76,
				48,
			},
		},
		4,
	},
	{
		25,
		high,
		dataEncoderType10To26,
		[]block{
			{
				7,
				54,
				24,
			},
			{
				22,
				55,
				25,
			},
		},
		4,
	},
	{
		25,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				22,
				45,
				15,
			},
			{
				13,
				46,
				16,
			},
		},
		4,
	},
	{
		26,
		low,
		dataEncoderType10To26,
		[]block{
			{
				10,
				142,
				114,
			},
			{
				2,
				143,
				115,
			},
		},
		4,
	},
	{
		26,
		medium,
		dataEncoderType10To26,
		[]block{
			{
				19,
				74,
				46,
			},
			{
				4,
				75,
				47,
			},
		},
		4,
	},
	{
		26,
		high,
		dataEncoderType10To26,
		[]block{
			{
				28,
				50,
				22,
			},
			{
				6,
				51,
				23,
			},
		},
		4,
	},
	{
		26,
		highest,
		dataEncoderType10To26,
		[]block{
			{
				33,
				46,
				16,
			},
			{
				4,
				47,
				17,
			},
		},
		4,
	},
	{
		27,
		low,
		dataEncoderType27To40,
		[]block{
			{
				8,
				152,
				122,
			},
			{
				4,
				153,
				123,
			},
		},
		4,
	},
	{
		27,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				22,
				73,
				45,
			},
			{
				3,
				74,
				46,
			},
		},
		4,
	},
	{
		27,
		high,
		dataEncoderType27To40,
		[]block{
			{
				8,
				53,
				23,
			},
			{
				26,
				54,
				24,
			},
		},
		4,
	},
	{
		27,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				12,
				45,
				15,
			},
			{
				28,
				46,
				16,
			},
		},
		4,
	},
	{
		28,
		low,
		dataEncoderType27To40,
		[]block{
			{
				3,
				147,
				117,
			},
			{
				10,
				148,
				118,
			},
		},
		3,
	},
	{
		28,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				3,
				73,
				45,
			},
			{
				23,
				74,
				46,
			},
		},
		3,
	},
	{
		28,
		high,
		dataEncoderType27To40,
		[]block{
			{
				4,
				54,
				24,
			},
			{
				31,
				55,
				25,
			},
		},
		3,
	},
	{
		28,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				11,
				45,
				15,
			},
			{
				31,
				46,
				16,
			},
		},
		3,
	},
	{
		29,
		low,
		dataEncoderType27To40,
		[]block{
			{
				7,
				146,
				116,
			},
			{
				7,
				147,
				117,
			},
		},
		3,
	},
	{
		29,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				21,
				73,
				45,
			},
			{
				7,
				74,
				46,
			},
		},
		3,
	},
	{
		29,
		high,
		dataEncoderType27To40,
		[]block{
			{
				1,
				53,
				23,
			},
			{
				37,
				54,
				24,
			},
		},
		3,
	},
	{
		29,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				19,
				45,
				15,
			},
			{
				26,
				46,
				16,
			},
		},
		3,
	},
	{
		30,
		low,
		dataEncoderType27To40,
		[]block{
			{
				5,
				145,
				115,
			},
			{
				10,
				146,
				116,
			},
		},
		3,
	},
	{
		30,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				19,
				75,
				47,
			},
			{
				10,
				76,
				48,
			},
		},
		3,
	},
	{
		30,
		high,
		dataEncoderType27To40,
		[]block{
			{
				15,
				54,
				24,
			},
			{
				25,
				55,
				25,
			},
		},
		3,
	},
	{
		30,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				23,
				45,
				15,
			},
			{
				25,
				46,
				16,
			},
		},
		3,
	},
	{
		31,
		low,
		dataEncoderType27To40,
		[]block{
			{
				13,
				145,
				115,
			},
			{
				3,
				146,
				116,
			},
		},
		3,
	},
	{
		31,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				2,
				74,
				46,
			},
			{
				29,
				75,
				47,
			},
		},
		3,
	},
	{
		31,
		high,
		dataEncoderType27To40,
		[]block{
			{
				42,
				54,
				24,
			},
			{
				1,
				55,
				25,
			},
		},
		3,
	},
	{
		31,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				23,
				45,
				15,
			},
			{
				28,
				46,
				16,
			},
		},
		3,
	},
	{
		32,
		low,
		dataEncoderType27To40,
		[]block{
			{
				17,
				145,
				115,
			},
		},
		3,
	},
	{
		32,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				10,
				74,
				46,
			},
			{
				23,
				75,
				47,
			},
		},
		3,
	},
	{
		32,
		high,
		dataEncoderType27To40,
		[]block{
			{
				10,
				54,
				24,
			},
			{
				35,
				55,
				25,
			},
		},
		3,
	},
	{
		32,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				19,
				45,
				15,
			},
			{
				35,
				46,
				16,
			},
		},
		3,
	},
	{
		33,
		low,
		dataEncoderType27To40,
		[]block{
			{
				17,
				145,
				115,
			},
			{
				1,
				146,
				116,
			},
		},
		3,
	},
	{
		33,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				14,
				74,
				46,
			},
			{
				21,
				75,
				47,
			},
		},
		3,
	},
	{
		33,
		high,
		dataEncoderType27To40,
		[]block{
			{
				29,
				54,
				24,
			},
			{
				19,
				55,
				25,
			},
		},
		3,
	},
	{
		33,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				11,
				45,
				15,
			},
			{
				46,
				46,
				16,
			},
		},
		3,
	},
	{
		34,
		low,
		dataEncoderType27To40,
		[]block{
			{
				13,
				145,
				115,
			},
			{
				6,
				146,
				116,
			},
		},
		3,
	},
	{
		34,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				14,
				74,
				46,
			},
			{
				23,
				75,
				47,
			},
		},
		3,
	},
	{
		34,
		high,
		dataEncoderType27To40,
		[]block{
			{
				44,
				54,
				24,
			},
			{
				7,
				55,
				25,
			},
		},
		3,
	},
	{
		34,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				59,
				46,
				16,
			},
			{
				1,
				47,
				17,
			},
		},
		3,
	},
	{
		35,
		low,
		dataEncoderType27To40,
		[]block{
			{
				12,
				151,
				121,
			},
			{
				7,
				152,
				122,
			},
		},
		0,
	},
	{
		35,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				12,
				75,
				47,
			},
			{
				26,
				76,
				48,
			},
		},
		0,
	},
	{
		35,
		high,
		dataEncoderType27To40,
		[]block{
			{
				39,
				54,
				24,
			},
			{
				14,
				55,
				25,
			},
		},
		0,
	},
	{
		35,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				22,
				45,
				15,
			},
			{
				41,
				46,
				16,
			},
		},
		0,
	},
	{
		36,
		low,
		dataEncoderType27To40,
		[]block{
			{
				6,
				151,
				121,
			},
			{
				14,
				152,
				122,
			},
		},
		0,
	},
	{
		36,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				6,
				75,
				47,
			},
			{
				34,
				76,
				48,
			},
		},
		0,
	},
	{
		36,
		high,
		dataEncoderType27To40,
		[]block{
			{
				46,
				54,
				24,
			},
			{
				10,
				55,
				25,
			},
		},
		0,
	},
	{
		36,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				2,
				45,
				15,
			},
			{
				64,
				46,
				16,
			},
		},
		0,
	},
	{
		37,
		low,
		dataEncoderType27To40,
		[]block{
			{
				17,
				152,
				122,
			},
			{
				4,
				153,
				123,
			},
		},
		0,
	},
	{
		37,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				29,
				74,
				46,
			},
			{
				14,
				75,
				47,
			},
		},
		0,
	},
	{
		37,
		high,
		dataEncoderType27To40,
		[]block{
			{
				49,
				54,
				24,
			},
			{
				10,
				55,
				25,
			},
		},
		0,
	},
	{
		37,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				24,
				45,
				15,
			},
			{
				46,
				46,
				16,
			},
		},
		0,
	},
	{
		38,
		low,
		dataEncoderType27To40,
		[]block{
			{
				4,
				152,
				122,
			},
			{
				18,
				153,
				123,
			},
		},
		0,
	},
	{
		38,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				13,
				74,
				46,
			},
			{
				32,
				75,
				47,
			},
		},
		0,
	},
	{
		38,
		high,
		dataEncoderType27To40,
		[]block{
			{
				48,
				54,
				24,
			},
			{
				14,
				55,
				25,
			},
		},
		0,
	},
	{
		38,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				42,
				45,
				15,
			},
			{
				32,
				46,
				16,
			},
		},
		0,
	},
	{
		39,
		low,
		dataEncoderType27To40,
		[]block{
			{
				20,
				147,
				117,
			},
			{
				4,
				148,
				118,
			},
		},
		0,
	},
	{
		39,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				40,
				75,
				47,
			},
			{
				7,
				76,
				48,
			},
		},
		0,
	},
	{
		39,
		high,
		dataEncoderType27To40,
		[]block{
			{
				43,
				54,
				24,
			},
			{
				22,
				55,
				25,
			},
		},
		0,
	},
	{
		39,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				10,
				45,
				15,
			},
			{
				67,
				46,
				16,
			},
		},
		0,
	},
	{
		40,
		low,
		dataEncoderType27To40,
		[]block{
			{
				19,
				148,
				118,
			},
			{
				6,
				149,
				119,
			},
		},
		0,
	},
	{
		40,
		medium,
		dataEncoderType27To40,
		[]block{
			{
				18,
				75,
				47,
			},
			{
				31,
				76,
				48,
			},
		},
		0,
	},
	{
		40,
		high,
		dataEncoderType27To40,
		[]block{
			{
				34,
				54,
				24,
			},
			{
				34,
				55,
				25,
			},
		},
		0,
	},
	{
		40,
		highest,
		dataEncoderType27To40,
		[]block{
			{
				20,
				45,
				15,
			},
			{
				61,
				46,
				16,
			},
		},
		0,
	},
}

var (
	formatBitSequence = []struct {
		regular uint32
		micro   uint32
	}{
		{0x5412, 0x4445},
		{0x5125, 0x4172},
		{0x5e7c, 0x4e2b},
		{0x5b4b, 0x4b1c},
		{0x45f9, 0x55ae},
		{0x40ce, 0x5099},
		{0x4f97, 0x5fc0},
		{0x4aa0, 0x5af7},
		{0x77c4, 0x6793},
		{0x72f3, 0x62a4},
		{0x7daa, 0x6dfd},
		{0x789d, 0x68ca},
		{0x662f, 0x7678},
		{0x6318, 0x734f},
		{0x6c41, 0x7c16},
		{0x6976, 0x7921},
		{0x1689, 0x06de},
		{0x13be, 0x03e9},
		{0x1ce7, 0x0cb0},
		{0x19d0, 0x0987},
		{0x0762, 0x1735},
		{0x0255, 0x1202},
		{0x0d0c, 0x1d5b},
		{0x083b, 0x186c},
		{0x355f, 0x2508},
		{0x3068, 0x203f},
		{0x3f31, 0x2f66},
		{0x3a06, 0x2a51},
		{0x24b4, 0x34e3},
		{0x2183, 0x31d4},
		{0x2eda, 0x3e8d},
		{0x2bed, 0x3bba},
	}
	versionBitSequence = []uint32{
		0x00000,
		0x00000,
		0x00000,
		0x00000,
		0x00000,
		0x00000,
		0x00000,
		0x07c94,
		0x085bc,
		0x09a99,
		0x0a4d3,
		0x0bbf6,
		0x0c762,
		0x0d847,
		0x0e60d,
		0x0f928,
		0x10b78,
		0x1145d,
		0x12a17,
		0x13532,
		0x149a6,
		0x15683,
		0x168c9,
		0x177ec,
		0x18ec4,
		0x191e1,
		0x1afab,
		0x1b08e,
		0x1cc1a,
		0x1d33f,
		0x1ed75,
		0x1f250,
		0x209d5,
		0x216f0,
		0x228ba,
		0x2379f,
		0x24b0b,
		0x2542e,
		0x26a64,
		0x27541,
		0x28c69,
	}
)

func (v qrCodeVersion) formatInfo(maskPattern int) *bitSet {
	formatID := 0

	switch v.level {
	case low:
		formatID = 0x08 // 0b01000
	case medium:
		formatID = 0x00 // 0b00000
	case high:
		formatID = 0x18 // 0b11000
	case highest:
		formatID = 0x10 // 0b10000
	default:
		panic("Invalid level")
	}

	if maskPattern < 0 || maskPattern > 7 {
		panic("Invalid maskPattern")
	}

	formatID |= maskPattern & 0x7

	result := bitsetnew()

	result.AppendUint32(formatBitSequence[formatID].regular, formatInfoLengthBits)

	return result
}

func (v qrCodeVersion) versionInfo() *bitSet {
	if v.version < 7 {
		return nil
	}

	result := bitsetnew()
	result.AppendUint32(versionBitSequence[v.version], 18)

	return result
}

func (v qrCodeVersion) numDataBits() int {
	numDataBits := 0
	for _, b := range v.block {
		numDataBits += 8 * b.numBlocks * b.numDataCodewords // 8 bits in a byte
	}

	return numDataBits
}

func chooseqrCodeVersion(level recoveryLevel, encoder *dataEncoder, numDataBits int) *qrCodeVersion {
	var chosenVersion *qrCodeVersion

	for _, v := range versions {
		if v.level != level {
			continue
		} else if v.version < encoder.minVersion {
			continue
		} else if v.version > encoder.maxVersion {
			break
		}

		numFreeBits := v.numDataBits() - numDataBits

		if numFreeBits >= 0 {
			v := v
			chosenVersion = &v
			break
		}
	}

	return chosenVersion
}

func (v qrCodeVersion) numTerminatorBitsRequired(numDataBits int) int {
	numFreeBits := v.numDataBits() - numDataBits

	var numTerminatorBits int

	switch {
	case numFreeBits >= 4:
		numTerminatorBits = 4
	default:
		numTerminatorBits = numFreeBits
	}

	return numTerminatorBits
}

func (v qrCodeVersion) numBlocks() int {
	numBlocks := 0

	for _, b := range v.block {
		numBlocks += b.numBlocks
	}

	return numBlocks
}

func (v qrCodeVersion) numBitsToPadToCodeword(numDataBits int) int {
	if numDataBits == v.numDataBits() {
		return 0
	}

	return (8 - numDataBits%8) % 8
}

func (v qrCodeVersion) symbolSize() int {
	return 21 + (v.version-1)*4
}

func (v qrCodeVersion) quietZoneSize() int {
	return 4
}

// condensed bitset package
type bitSet struct {
	numBits int
	bits    []byte
}

func bitsetnew(v ...bool) *bitSet {
	b := &bitSet{numBits: 0, bits: make([]byte, 0)}
	b.AppendBools(v...)

	return b
}

func bitsetClone(from *bitSet) *bitSet {
	return &bitSet{numBits: from.numBits, bits: from.bits[:]}
}

func (b *bitSet) Substr(start, end int) *bitSet {
	if start > end || end > b.numBits {
		panic("out of range")
	}

	result := bitsetnew()
	result.ensureCapacity(end - start)

	for i := start; i < end; i++ {
		if b.At(i) {
			result.bits[result.numBits/8] |= 0x80 >> uint(result.numBits%8)
		}
		result.numBits++
	}

	return result
}

func (b *bitSet) AppendBytes(data []byte) {
	for _, d := range data {
		b.AppendByte(d, 8)
	}
}

func (b *bitSet) AppendByte(value byte, numBits int) {
	b.ensureCapacity(numBits)

	if numBits > 8 {
		panic("numBits out of range")
	}

	for i := numBits - 1; i >= 0; i-- {
		if value&(1<<uint(i)) != 0 {
			b.bits[b.numBits/8] |= 0x80 >> uint(b.numBits%8)
		}

		b.numBits++
	}
}

func (b *bitSet) AppendUint32(value uint32, numBits int) {
	b.ensureCapacity(numBits)

	if numBits > 32 {
		panic("numBits out of range 0-32")
	}

	for i := numBits - 1; i >= 0; i-- {
		if value&(1<<uint(i)) != 0 {
			b.bits[b.numBits/8] |= 0x80 >> uint(b.numBits%8)
		}

		b.numBits++
	}
}

func (b *bitSet) ensureCapacity(numBits int) {
	numBits += b.numBits

	newNumBytes := numBits / 8
	if numBits%8 != 0 {
		newNumBytes++
	}

	if len(b.bits) >= newNumBytes {
		return
	}

	b.bits = append(b.bits, make([]byte, newNumBytes+2*len(b.bits))...)
}

func (b *bitSet) Append(other *bitSet) {
	b.ensureCapacity(other.numBits)

	for i := 0; i < other.numBits; i++ {
		if other.At(i) {
			b.bits[b.numBits/8] |= 0x80 >> uint(b.numBits%8)
		}
		b.numBits++
	}
}

func (b *bitSet) AppendBools(bits ...bool) {
	b.ensureCapacity(len(bits))

	for _, v := range bits {
		if v {
			b.bits[b.numBits/8] |= 0x80 >> uint(b.numBits%8)
		}
		b.numBits++
	}
}

func (b *bitSet) AppendNumBools(num int, value bool) {
	for i := 0; i < num; i++ {
		b.AppendBools(value)
	}
}

func (b *bitSet) String() string {
	var bitString string
	for i := 0; i < b.numBits; i++ {
		if (i % 8) == 0 {
			bitString += " "
		}

		if (b.bits[i/8] & (0x80 >> byte(i%8))) != 0 {
			bitString += "1"
		} else {
			bitString += "0"
		}
	}

	return "error bits"
}

func (b *bitSet) Len() int {
	return b.numBits
}

func (b *bitSet) Bits() []bool {
	result := make([]bool, b.numBits)

	var i int
	for i = 0; i < b.numBits; i++ {
		result[i] = (b.bits[i/8] & (0x80 >> byte(i%8))) != 0
	}

	return result
}

func (b *bitSet) At(index int) bool {
	if index >= b.numBits {
		panic("Index out of range")
	}

	return (b.bits[index/8] & (0x80 >> byte(index%8))) != 0
}

func (b *bitSet) Equals(other *bitSet) bool {
	if b.numBits != other.numBits {
		return false
	}

	if !bytes.Equal(b.bits[0:b.numBits/8], other.bits[0:b.numBits/8]) {
		return false
	}

	for i := 8 * (b.numBits / 8); i < b.numBits; i++ {
		a := (b.bits[i/8] & (0x80 >> byte(i%8)))
		b := (other.bits[i/8] & (0x80 >> byte(i%8)))

		if a != b {
			return false
		}
	}

	return true
}

func (b *bitSet) ByteAt(index int) byte {
	if index < 0 || index >= b.numBits {
		panic("Index out of range")
	}

	var result byte

	for i := index; i < index+8 && i < b.numBits; i++ {
		result <<= 1
		if b.At(i) {
			result |= 1
		}
	}

	return result
}

// condensed forward error correction redsolomon
const (
	gfZero = gfElement(0)
	gfOne  = gfElement(1)
)

var (
	gfExpTable = [256]gfElement{
		/*   0 -   9 */ 1, 2, 4, 8, 16, 32, 64, 128, 29, 58,
		/*  10 -  19 */ 116, 232, 205, 135, 19, 38, 76, 152, 45, 90,
		/*  20 -  29 */ 180, 117, 234, 201, 143, 3, 6, 12, 24, 48,
		/*  30 -  39 */ 96, 192, 157, 39, 78, 156, 37, 74, 148, 53,
		/*  40 -  49 */ 106, 212, 181, 119, 238, 193, 159, 35, 70, 140,
		/*  50 -  59 */ 5, 10, 20, 40, 80, 160, 93, 186, 105, 210,
		/*  60 -  69 */ 185, 111, 222, 161, 95, 190, 97, 194, 153, 47,
		/*  70 -  79 */ 94, 188, 101, 202, 137, 15, 30, 60, 120, 240,
		/*  80 -  89 */ 253, 231, 211, 187, 107, 214, 177, 127, 254, 225,
		/*  90 -  99 */ 223, 163, 91, 182, 113, 226, 217, 175, 67, 134,
		/* 100 - 109 */ 17, 34, 68, 136, 13, 26, 52, 104, 208, 189,
		/* 110 - 119 */ 103, 206, 129, 31, 62, 124, 248, 237, 199, 147,
		/* 120 - 129 */ 59, 118, 236, 197, 151, 51, 102, 204, 133, 23,
		/* 130 - 139 */ 46, 92, 184, 109, 218, 169, 79, 158, 33, 66,
		/* 140 - 149 */ 132, 21, 42, 84, 168, 77, 154, 41, 82, 164,
		/* 150 - 159 */ 85, 170, 73, 146, 57, 114, 228, 213, 183, 115,
		/* 160 - 169 */ 230, 209, 191, 99, 198, 145, 63, 126, 252, 229,
		/* 170 - 179 */ 215, 179, 123, 246, 241, 255, 227, 219, 171, 75,
		/* 180 - 189 */ 150, 49, 98, 196, 149, 55, 110, 220, 165, 87,
		/* 190 - 199 */ 174, 65, 130, 25, 50, 100, 200, 141, 7, 14,
		/* 200 - 209 */ 28, 56, 112, 224, 221, 167, 83, 166, 81, 162,
		/* 210 - 219 */ 89, 178, 121, 242, 249, 239, 195, 155, 43, 86,
		/* 220 - 229 */ 172, 69, 138, 9, 18, 36, 72, 144, 61, 122,
		/* 230 - 239 */ 244, 245, 247, 243, 251, 235, 203, 139, 11, 22,
		/* 240 - 249 */ 44, 88, 176, 125, 250, 233, 207, 131, 27, 54,
		/* 250 - 255 */ 108, 216, 173, 71, 142, 1,
	}

	gfLogTable = [256]int{
		/*   0 -   9 */ -1, 0, 1, 25, 2, 50, 26, 198, 3, 223,
		/*  10 -  19 */ 51, 238, 27, 104, 199, 75, 4, 100, 224, 14,
		/*  20 -  29 */ 52, 141, 239, 129, 28, 193, 105, 248, 200, 8,
		/*  30 -  39 */ 76, 113, 5, 138, 101, 47, 225, 36, 15, 33,
		/*  40 -  49 */ 53, 147, 142, 218, 240, 18, 130, 69, 29, 181,
		/*  50 -  59 */ 194, 125, 106, 39, 249, 185, 201, 154, 9, 120,
		/*  60 -  69 */ 77, 228, 114, 166, 6, 191, 139, 98, 102, 221,
		/*  70 -  79 */ 48, 253, 226, 152, 37, 179, 16, 145, 34, 136,
		/*  80 -  89 */ 54, 208, 148, 206, 143, 150, 219, 189, 241, 210,
		/*  90 -  99 */ 19, 92, 131, 56, 70, 64, 30, 66, 182, 163,
		/* 100 - 109 */ 195, 72, 126, 110, 107, 58, 40, 84, 250, 133,
		/* 110 - 119 */ 186, 61, 202, 94, 155, 159, 10, 21, 121, 43,
		/* 120 - 129 */ 78, 212, 229, 172, 115, 243, 167, 87, 7, 112,
		/* 130 - 139 */ 192, 247, 140, 128, 99, 13, 103, 74, 222, 237,
		/* 140 - 149 */ 49, 197, 254, 24, 227, 165, 153, 119, 38, 184,
		/* 150 - 159 */ 180, 124, 17, 68, 146, 217, 35, 32, 137, 46,
		/* 160 - 169 */ 55, 63, 209, 91, 149, 188, 207, 205, 144, 135,
		/* 170 - 179 */ 151, 178, 220, 252, 190, 97, 242, 86, 211, 171,
		/* 180 - 189 */ 20, 42, 93, 158, 132, 60, 57, 83, 71, 109,
		/* 190 - 199 */ 65, 162, 31, 45, 67, 216, 183, 123, 164, 118,
		/* 200 - 209 */ 196, 23, 73, 236, 127, 12, 111, 246, 108, 161,
		/* 210 - 219 */ 59, 82, 41, 157, 85, 170, 251, 96, 134, 177,
		/* 220 - 229 */ 187, 204, 62, 90, 203, 89, 95, 176, 156, 169,
		/* 230 - 239 */ 160, 81, 11, 245, 22, 235, 122, 117, 44, 215,
		/* 240 - 249 */ 79, 174, 213, 233, 230, 231, 173, 232, 116, 214,
		/* 250 - 255 */ 244, 234, 168, 80, 88, 175,
	}
)

type gfElement uint8

func gfAdd(a, b gfElement) gfElement {
	return a ^ b
}

func gfMultiply(a, b gfElement) gfElement {
	if a == gfZero || b == gfZero {
		return gfZero
	}

	return gfExpTable[(gfLogTable[a]+gfLogTable[b])%255]
}

func gfDivide(a, b gfElement) gfElement {
	if a == gfZero {
		return gfZero
	} else if b == gfZero {
		panic("divide by zero")
	}

	return gfMultiply(a, gfInverse(b))
}

func gfInverse(a gfElement) gfElement {
	if a == gfZero {
		panic("no multiplicative inverse of 0")
	}

	return gfExpTable[255-gfLogTable[a]]
}

type gfPoly struct {
	term []gfElement
}

func newGFPolyFromData(data *bitSet) gfPoly {
	numTotalBytes := data.Len() / 8
	if data.Len()%8 != 0 {
		numTotalBytes++
	}

	result := gfPoly{term: make([]gfElement, numTotalBytes)}

	i := numTotalBytes - 1
	for j := 0; j < data.Len(); j += 8 {
		result.term[i] = gfElement(data.ByteAt(j))
		i--
	}

	return result
}

func newGFPolyMonomial(term gfElement, degree int) gfPoly {
	if term == gfZero {
		return gfPoly{}
	}

	result := gfPoly{term: make([]gfElement, degree+1)}
	result.term[degree] = term

	return result
}

func (e gfPoly) data(numTerms int) []byte {
	result := make([]byte, numTerms)

	i := numTerms - len(e.term)
	for j := len(e.term) - 1; j >= 0; j-- {
		result[i] = byte(e.term[j])
		i++
	}

	return result
}

func (e gfPoly) numTerms() int {
	return len(e.term)
}

func gfPolyMultiply(a, b gfPoly) gfPoly {
	numATerms := a.numTerms()
	numBTerms := b.numTerms()
	result := gfPoly{term: make([]gfElement, numATerms+numBTerms)}
	for i := 0; i < numATerms; i++ {
		for j := 0; j < numBTerms; j++ {
			if a.term[i] != 0 && b.term[j] != 0 {
				monomial := gfPoly{term: make([]gfElement, i+j+1)}
				monomial.term[i+j] = gfMultiply(a.term[i], b.term[j])

				result = gfPolyAdd(result, monomial)
			}
		}
	}
	return result.normalised()
}

func gfPolyRemainder(numerator, denominator gfPoly) gfPoly {
	if denominator.equals(gfPoly{}) {
		panic("remainder by zero")
	}
	remainder := numerator
	for remainder.numTerms() >= denominator.numTerms() {
		degree := remainder.numTerms() - denominator.numTerms()
		coefficient := gfDivide(remainder.term[remainder.numTerms()-1],
			denominator.term[denominator.numTerms()-1])

		divisor := gfPolyMultiply(denominator,
			newGFPolyMonomial(coefficient, degree))

		remainder = gfPolyAdd(remainder, divisor)
	}
	return remainder.normalised()
}

func gfPolyAdd(a, b gfPoly) gfPoly {
	numATerms := a.numTerms()
	numBTerms := b.numTerms()
	numTerms := numATerms
	if numBTerms > numTerms {
		numTerms = numBTerms
	}
	result := gfPoly{term: make([]gfElement, numTerms)}
	for i := 0; i < numTerms; i++ {
		switch {
		case numATerms > i && numBTerms > i:
			result.term[i] = gfAdd(a.term[i], b.term[i])
		case numATerms > i:
			result.term[i] = a.term[i]
		default:
			result.term[i] = b.term[i]
		}
	}
	return result.normalised()
}

func (e gfPoly) normalised() gfPoly {
	numTerms := e.numTerms()
	maxNonzeroTerm := numTerms - 1
	for i := numTerms - 1; i >= 0; i-- {
		if e.term[i] != 0 {
			break
		}

		maxNonzeroTerm = i - 1
	}
	if maxNonzeroTerm < 0 {
		return gfPoly{}
	} else if maxNonzeroTerm < numTerms-1 {
		e.term = e.term[0 : maxNonzeroTerm+1]
	}
	return e
}

func (e gfPoly) equals(other gfPoly) bool {
	var minecPoly *gfPoly
	var maxecPoly *gfPoly

	if e.numTerms() > other.numTerms() {
		minecPoly = &other
		maxecPoly = &e
	} else {
		minecPoly = &e
		maxecPoly = &other
	}

	numMinTerms := minecPoly.numTerms()
	numMaxTerms := maxecPoly.numTerms()

	for i := 0; i < numMinTerms; i++ {
		if e.term[i] != other.term[i] {
			return false
		}
	}

	for i := numMinTerms; i < numMaxTerms; i++ {
		if maxecPoly.term[i] != 0 {
			return false
		}
	}

	return true
}

func rEncode(data *bitSet, numECBytes int) *bitSet {
	ecpoly := newGFPolyFromData(data)
	ecpoly = gfPolyMultiply(ecpoly, newGFPolyMonomial(gfOne, numECBytes))
	generator := rsGeneratorPoly(numECBytes)
	remainder := gfPolyRemainder(ecpoly, generator)
	result := bitsetClone(data)
	result.AppendBytes(remainder.data(numECBytes))

	return result
}

func rsGeneratorPoly(degree int) gfPoly {
	if degree < 2 {
		panic("degree < 2")
	}
	generator := gfPoly{term: []gfElement{1}}
	for i := 0; i < degree; i++ {
		nextPoly := gfPoly{term: []gfElement{gfExpTable[i], 1}}
		generator = gfPolyMultiply(generator, nextPoly)
	}
	return generator
}
