// Package imgcompose склеивает несколько картинок в одну — с номерами и стрелками.
//
// Зачем: в Telegram у сообщения одно фото, а на телефоне альбом режется в мелкие
// превью. Одна вертикальная картинка открывается на весь экран, масштабируется
// пальцами, а пара «условие → ответ» всегда лежит в одной строке и не путается.
// Только стандартная библиотека.
package imgcompose

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"  // регистрирует декодер
	_ "image/jpeg" // регистрирует декодер
	"image/png"
	"io"
	"os"
)

// Version меняйте при любом изменении внешнего вида склейки: она входит в ключ кэша
// готовых файлов, и старые картинки перестанут использоваться.
const Version = 1

// Row — одна строка результата: [номер] [левые картинки] → [правые картинки].
// Картинки в ячейке идут друг под другом.
type Row struct {
	Label       int      // 0 — без номера
	Left, Right []string // пути к файлам
}

const (
	pad        = 24
	badgeCol   = 84  // ширина колонки номера
	singleColW = 800 // ширина ячейки, когда правых картинок нет
	pairColW   = 380 // ширина каждой ячейки в режиме «пары»
	arrowW     = 72
	maxUpscale = 3.0
	maxCellH   = 900
	cellGap    = 14
	rowGap     = 32
	digitScale = 6
	maxSide    = 8000     // отсекаем битые заголовки (в банке есть файлы с «шириной» в миллионы)
	maxPixels  = 16 << 20 // и слишком тяжёлые картинки
	maxCanvasH = 6000     // Telegram: ширина+высота фото <= 10000
)

var (
	white  = color.RGBA{255, 255, 255, 255}
	ink    = color.RGBA{31, 58, 95, 255}
	grey   = color.RGBA{200, 205, 212, 255}
	arrowC = color.RGBA{90, 98, 110, 255}
)

type cell struct {
	imgs []*image.RGBA
	h    int
}

// Compose собирает картинку. Файлы, которые не удалось прочитать, пропускаются
// (их описание — в skipped). Если рисовать нечего — возвращает nil.
func Compose(rows []Row) (img *image.RGBA, skipped []error) {
	hasRight, hasLabel := false, false
	for _, r := range rows {
		hasRight = hasRight || len(r.Right) > 0
		hasLabel = hasLabel || r.Label > 0
	}
	colW := singleColW
	if hasRight {
		colW = pairColW
	}

	type prepared struct {
		label       int
		left, right cell
		h           int
	}
	var prep []prepared
	for _, r := range rows {
		p := prepared{label: r.Label}
		p.left = loadCell(r.Left, colW, &skipped)
		p.right = loadCell(r.Right, colW, &skipped)
		if len(p.left.imgs)+len(p.right.imgs) == 0 {
			continue
		}
		p.h = max(p.left.h, p.right.h)
		if p.label > 0 {
			p.h = max(p.h, badgeH())
		}
		prep = append(prep, p)
	}
	if len(prep) == 0 {
		return nil, skipped
	}

	w := 2*pad + colW
	if hasLabel {
		w += badgeCol
	}
	if hasRight {
		w += arrowW + colW
	}
	h := 2 * pad
	for _, p := range prep {
		h += p.h
	}
	h += rowGap * (len(prep) - 1)

	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(white), image.Point{}, draw.Src)

	y := pad
	for i, p := range prep {
		x := pad
		if hasLabel {
			if p.label > 0 {
				drawBadge(canvas, x, y, p.label)
			}
			x += badgeCol
		}
		drawCell(canvas, p.left, x, y, colW)
		if hasRight {
			if len(p.left.imgs) > 0 && len(p.right.imgs) > 0 {
				drawArrow(canvas, x+colW, y+p.h/2, arrowW)
			}
			drawCell(canvas, p.right, x+colW+arrowW, y, colW)
		}
		y += p.h
		if i < len(prep)-1 {
			ly := y + rowGap/2
			hline(canvas, pad, w-pad, ly, 2, grey)
			y += rowGap
		}
	}

	if h > maxCanvasH {
		f := float64(maxCanvasH) / float64(h)
		canvas = resize(canvas, max(1, int(float64(w)*f)), maxCanvasH)
	}
	return canvas, skipped
}

// WritePNG кодирует картинку в w.
func WritePNG(w io.Writer, img image.Image) error {
	return (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(w, img)
}

func loadCell(paths []string, colW int, skipped *[]error) cell {
	var c cell
	for _, p := range paths {
		src, err := load(p)
		if err != nil {
			*skipped = append(*skipped, err)
			continue
		}
		b := src.Bounds()
		scale := min(float64(colW)/float64(b.Dx()), float64(maxCellH)/float64(b.Dy()), maxUpscale)
		w := max(1, int(float64(b.Dx())*scale+0.5))
		h := max(1, int(float64(b.Dy())*scale+0.5))
		c.imgs = append(c.imgs, resize(src, w, h))
		if c.h > 0 {
			c.h += cellGap
		}
		c.h += h
	}
	return c
}

// load читает файл, отсекая битые заголовки, и заливает прозрачность белым.
func load(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxSide || cfg.Height > maxSide || cfg.Width*cfg.Height > maxPixels {
		return nil, fmt.Errorf("%s: неподходящий размер %dx%d (%s)", path, cfg.Width, cfg.Height, format)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	dst := image.NewRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), image.NewUniform(white), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Over)
	return dst, nil
}

func drawCell(dst *image.RGBA, c cell, x, y, colW int) {
	for _, im := range c.imgs {
		b := im.Bounds()
		ox := x + (colW-b.Dx())/2
		draw.Draw(dst, image.Rect(ox, y, ox+b.Dx(), y+b.Dy()), im, b.Min, draw.Src)
		y += b.Dy() + cellGap
	}
}

// resize — ресемплинг: билинейный при увеличении, усреднение по площади при уменьшении.
func resize(src *image.RGBA, w, h int) *image.RGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	fx, fy := float64(sw)/float64(w), float64(sh)/float64(h)

	at := func(x, y int) (float64, float64, float64) {
		x = min(max(x, 0), sw-1)
		y = min(max(y, 0), sh-1)
		o := src.PixOffset(sb.Min.X+x, sb.Min.Y+y)
		return float64(src.Pix[o]), float64(src.Pix[o+1]), float64(src.Pix[o+2])
	}

	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			var r, g, b float64
			if fx <= 1 && fy <= 1 { // увеличение — билинейно
				sx, sy := (float64(dx)+0.5)*fx-0.5, (float64(dy)+0.5)*fy-0.5
				x0, y0 := int(floor(sx)), int(floor(sy))
				tx, ty := sx-float64(x0), sy-float64(y0)
				r00, g00, b00 := at(x0, y0)
				r10, g10, b10 := at(x0+1, y0)
				r01, g01, b01 := at(x0, y0+1)
				r11, g11, b11 := at(x0+1, y0+1)
				lerp := func(a, b, t float64) float64 { return a + (b-a)*t }
				r = lerp(lerp(r00, r10, tx), lerp(r01, r11, tx), ty)
				g = lerp(lerp(g00, g10, tx), lerp(g01, g11, tx), ty)
				b = lerp(lerp(b00, b10, tx), lerp(b01, b11, tx), ty)
			} else { // уменьшение — среднее по покрываемому блоку
				x0, x1 := int(float64(dx)*fx), max(int(float64(dx+1)*fx), int(float64(dx)*fx)+1)
				y0, y1 := int(float64(dy)*fy), max(int(float64(dy+1)*fy), int(float64(dy)*fy)+1)
				n := 0.0
				for yy := y0; yy < y1; yy++ {
					for xx := x0; xx < x1; xx++ {
						pr, pg, pb := at(xx, yy)
						r, g, b, n = r+pr, g+pg, b+pb, n+1
					}
				}
				r, g, b = r/n, g/n, b/n
			}
			o := dst.PixOffset(dx, dy)
			dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2], dst.Pix[o+3] = uint8(r+0.5), uint8(g+0.5), uint8(b+0.5), 255
		}
	}
	return dst
}

func floor(v float64) float64 {
	i := float64(int(v))
	if v < i {
		return i - 1
	}
	return i
}

func hline(dst *image.RGBA, x0, x1, y, thick int, c color.RGBA) {
	draw.Draw(dst, image.Rect(x0, y, x1, y+thick), image.NewUniform(c), image.Point{}, draw.Src)
}

func drawArrow(dst *image.RGBA, x, cy, w int) {
	x0, x1 := x+12, x+w-12
	hline(dst, x0, x1-14, cy-3, 6, arrowC)
	// наконечник — залитый треугольник
	for i := 0; i < 18; i++ {
		half := 18 - i
		draw.Draw(dst, image.Rect(x1-18+i, cy-half, x1-17+i, cy+half+1), image.NewUniform(arrowC), image.Point{}, draw.Src)
	}
}
