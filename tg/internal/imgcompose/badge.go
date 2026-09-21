package imgcompose

import (
	"image"
	"image/draw"
	"strconv"
)

// Цифры шрифтом 5x7: свои, чтобы не тянуть golang.org/x/image.
var digits = [10][7]uint8{
	{0b01110, 0b10001, 0b10011, 0b10101, 0b11001, 0b10001, 0b01110},
	{0b00100, 0b01100, 0b00100, 0b00100, 0b00100, 0b00100, 0b01110},
	{0b01110, 0b10001, 0b00001, 0b00010, 0b00100, 0b01000, 0b11111},
	{0b11110, 0b00001, 0b00001, 0b01110, 0b00001, 0b00001, 0b11110},
	{0b00010, 0b00110, 0b01010, 0b10010, 0b11111, 0b00010, 0b00010},
	{0b11111, 0b10000, 0b11110, 0b00001, 0b00001, 0b10001, 0b01110},
	{0b00110, 0b01000, 0b10000, 0b11110, 0b10001, 0b10001, 0b01110},
	{0b11111, 0b00001, 0b00010, 0b00100, 0b01000, 0b01000, 0b01000},
	{0b01110, 0b10001, 0b10001, 0b01110, 0b10001, 0b10001, 0b01110},
	{0b01110, 0b10001, 0b10001, 0b01111, 0b00001, 0b00010, 0b01100},
}

func badgeH() int { return 7*digitScale + 16 }

// drawBadge рисует номер белыми цифрами на тёмной плашке в точке (x, y).
func drawBadge(dst *image.RGBA, x, y, n int) {
	s := strconv.Itoa(n)
	w := len(s)*6*digitScale - digitScale + 20
	draw.Draw(dst, image.Rect(x, y, x+w, y+badgeH()), image.NewUniform(ink), image.Point{}, draw.Src)
	for i, ch := range s {
		glyph := digits[ch-'0']
		gx := x + 10 + i*6*digitScale
		for row := 0; row < 7; row++ {
			for col := 0; col < 5; col++ {
				if glyph[row]&(1<<(4-col)) != 0 {
					r := image.Rect(gx+col*digitScale, y+8+row*digitScale, gx+(col+1)*digitScale, y+8+(row+1)*digitScale)
					draw.Draw(dst, r, image.NewUniform(white), image.Point{}, draw.Src)
				}
			}
		}
	}
}
