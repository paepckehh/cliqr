// package cliqr ...
package cliqr

import (
// "os"
)

// QR ...
func QR(in string) string {
	l := len(in)
	switch {
	case l == 0:
		return "no intput"
	case l > 1280:
		return "input to large"
	case l > 245:
		return QRsmall(in)
	default:
		return QRbig(in)
	}
}

// QRbig ...
func QRbig(buf string) string {
	qr, err := newQR(buf, low)
	if err != nil {
		return ("unable to encode input to qr code: " + err.Error())
	}
	bits := qr.Bitmap()
	buf = ""
	for y := range bits {
		for x := range bits[y] {
			if bits[y][x] {
				buf += _blank_x2
				continue
			}
			buf += _block_x2
		}
		buf += _lf
	}
	return buf
}

// QRsmall ...
func QRsmall(buf string) string {
	qr, err := newQR(buf, low)
	if err != nil {
		return ("unable to encode input to qr code: " + err.Error())
	}
	bits := qr.Bitmap()
	buf = ""
	for y := 0; y < len(bits)-1; y += 2 {
		for x := range bits[y] {
			if bits[y][x] == bits[y+1][x] {
				if bits[y][x] {
					buf += _blank
					continue
				}
				buf += _block
				continue
			}
			if bits[y][x] {
				buf += _block_down
				continue
			}
			buf += _block_up
		}
		buf += _lf
	}
	if len(bits)%2 == 1 {
		y := len(bits) - 1
		for x := range bits[y] {
			if bits[y][x] {
				buf += _blank
				continue
			}
			buf += _block_up
		}
		buf += _lf
	}
	return buf
}
