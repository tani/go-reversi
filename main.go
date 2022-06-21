// (C) 2022 TANIGUCHI Masaya
// https://git.io/mit-license

package main

import (
	"bytes"
	"embed"
	"fmt"
	"image/color"
	_ "image/png"
	"math"
	"math/bits"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

//go:embed assets/*
var assets embed.FS

var blackImg, whiteImg *ebiten.Image
var fontFace font.Face

func init() {
	fontBin, _ := assets.ReadFile("assets/PressStart2P-Regular.ttf")
	fontTT, _ := opentype.Parse(fontBin)
	fontFace, _ = opentype.NewFace(fontTT, &opentype.FaceOptions{
		DPI:     72,
		Size:    10,
		Hinting: font.HintingFull,
	})
	blackBin, _ := assets.ReadFile("assets/black.png")
	whiteBin, _ := assets.ReadFile("assets/white.png")
	blackImg, _, _ = ebitenutil.NewImageFromReader(bytes.NewReader(blackBin))
	whiteImg, _, _ = ebitenutil.NewImageFromReader(bytes.NewReader(whiteBin))
}

const VerticalMask uint64 = 0x00ffffffffffff00
const HorizontalMask uint64 = 0x7e7e7e7e7e7e7e7e
const EdgeMask uint64 = ^VerticalMask | ^HorizontalMask
const CornerMask uint64 = ^VerticalMask & ^(VerticalMask ^ HorizontalMask)

func GetCandidates(black, white uint64) uint64 {
	mask := [4]uint64{
		white & HorizontalMask,
		white & HorizontalMask & VerticalMask,
		white & VerticalMask,
		white & HorizontalMask & VerticalMask,
	}
	diff := [4]uint64{1, 7, 8, 9}
	result1 := uint64(0)
	result2 := uint64(0)
	for i := 0; i < 4; i++ {
		pattern1 := mask[i] & (black << diff[i])
		pattern2 := mask[i] & (black >> diff[i])
		for j := 0; j < 5; j++ {
			pattern1 |= mask[i] & (pattern1 << diff[i])
			pattern2 |= mask[i] & (pattern2 >> diff[i])
		}
		result1 |= (pattern1 << diff[i])
		result2 |= (pattern2 >> diff[i])
	}
	return (result1 | result2) & ^(black | white)
}

func GetReverse(black, white, position uint64) uint64 {
	mask := [4]uint64{
		white & HorizontalMask,
		white & HorizontalMask & VerticalMask,
		white & VerticalMask,
		white & HorizontalMask & VerticalMask,
	}
	diff := [4]uint64{1, 7, 8, 9}
	result := uint64(0)
	for i := 0; i < 4; i++ {
		pattern1 := mask[i] & (black << diff[i])
		pattern2 := mask[i] & (position >> diff[i])
		pattern3 := mask[i] & (black >> diff[i])
		pattern4 := mask[i] & (position << diff[i])
		for j := 0; j < 5; j++ {
			pattern1 |= mask[i] & (pattern1 << diff[i])
			pattern2 |= mask[i] & (pattern2 >> diff[i])
			pattern3 |= mask[i] & (pattern3 >> diff[i])
			pattern4 |= mask[i] & (pattern4 << diff[i])
		}
		result |= (pattern1 & pattern2) | (pattern3 & pattern4)
	}
	return result
}

func Evaluate(black, white uint64, n, depth int, a, b float64) float64 {
	if n == 0 {
		blackCandidates := GetCandidates(black, white)
		whiteCandidates := GetCandidates(white, black)
		diff := bits.OnesCount64(blackCandidates) - bits.OnesCount64(whiteCandidates) + 64
		nedge := bits.OnesCount64(black&EdgeMask) - bits.OnesCount64(white&EdgeMask) + 28
		ncorner := bits.OnesCount64(black&CornerMask) - bits.OnesCount64(white&CornerMask) + 4
		return float64(diff+4*nedge+8*ncorner) / (1.0*128 + 4.0*56 + 8.0*8)
	}

	candidates := GetCandidates(white, black)
	m := bits.OnesCount64(candidates)
	for i := 0; i < m; i++ {
		position := uint64(1) << (63 - bits.LeadingZeros64(candidates))
		reverse := GetReverse(white, black, position)
		white := white ^ reverse ^ position
		black := black ^ reverse
		a = math.Max(a, -Evaluate(white, black, n-(depth%2), depth+1, -b, -a))
		if a >= b {
			return a
		}
	}
	return a
}

const (
	YOU = iota
	COM
)

const initialBlack = (uint64(1) << (8*3 + 4)) | (uint64(1) << (8*4 + 3))
const initialWhite = (uint64(1) << (8*4 + 4)) | (uint64(1) << (8*3 + 3))

type Game struct {
	cellSize     int
	boardSize    int
	boardMargin  int
	player       int
	black, white uint64
	lock         int64
}

func (game *Game) Update() error {
	if atomic.CompareAndSwapInt64(&game.lock, 0, 1) {
		return nil
	}
	defer atomic.StoreInt64(&game.lock, 0)

	cursorX, cursorY := ebiten.CursorPosition()

	ids := inpututil.AppendJustPressedTouchIDs([]ebiten.TouchID{})
	if len(ids) != 0 {
		cursorX, cursorY = ebiten.TouchPosition(ids[0])
	}

	if game.boardMargin+340 < cursorX && cursorX < game.boardMargin+400 {
		if 15 < cursorY && cursorY < 35 {
			if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
				game.black = initialBlack
				game.white = initialWhite
				return nil
			}
		}
	}

	if game.player == YOU {
		candidates := GetCandidates(game.black, game.white)
		if candidates == 0 {
			game.player = COM
			return nil
		}
		cursorX, cursorY := ebiten.CursorPosition()
		if !(game.boardMargin < cursorX && cursorX < game.boardSize+game.boardMargin) {
			if !(game.boardMargin < cursorY && cursorY < game.boardMargin+game.boardMargin) {
				return nil
			}
		}
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			positionX := (cursorX - game.boardMargin) / game.cellSize
			positionY := (cursorY - game.boardMargin) / game.cellSize
			position := uint64(1) << (positionX + positionY*8)
			if (position & candidates) > 0 {
				reverse := GetReverse(game.black, game.white, position)
				game.black ^= reverse ^ position
				game.white ^= reverse
				game.player = COM
			}
		}
	} else {
		candidates := GetCandidates(game.white, game.black)
		bestBlack := game.black
		bestWhite := game.white
		bestScore := 0.0
		for candidates > 0 {
			position := uint64(1) << (63 - bits.LeadingZeros64(candidates))
			reverse := GetReverse(game.white, game.black, position)
			white := game.white ^ reverse ^ position
			black := game.black ^ reverse
			score := Evaluate(white, black, 5, 0, 0, 1)
			if bestScore < score {
				bestBlack = black
				bestWhite = white
				bestScore = score
			}
			candidates -= position
		}
		game.black = bestBlack
		game.white = bestWhite
		game.player = YOU
	}
	return nil
}

func (game *Game) Draw(screen *ebiten.Image) {
	ebitenutil.DrawRect(screen, 0, 0, float64(game.boardSize+game.boardMargin*2), float64(game.boardSize+game.boardMargin*2), color.RGBA{0x00, 0xff, 0x00, 0xff})
	for i := 0; i <= 8; i++ {
		ebitenutil.DrawLine(screen, float64(game.cellSize*i+game.boardMargin), float64(game.boardMargin), float64(game.cellSize*i+game.boardMargin), float64(game.boardSize+game.boardMargin), color.Black)
		ebitenutil.DrawLine(screen, float64(game.boardMargin), float64(game.cellSize*i+game.boardMargin), float64(game.boardSize+game.boardMargin), float64(game.cellSize*i+game.boardMargin), color.Black)
	}
	msg := fmt.Sprintf("BLACK: %d WHITE: %d", bits.OnesCount64(game.black), bits.OnesCount64(game.white))
	text.Draw(screen, msg, fontFace, game.boardMargin, 30, color.Black)
	ebitenutil.DrawRect(screen, float64(game.boardMargin+340), 15, 60, 20, color.Black)
	text.Draw(screen, "RESET", fontFace, game.boardMargin+345, 30, color.RGBA{0x00, 0xff, 0x00, 0xff})
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			position := uint64(1) << (i + j*8)
			option := &ebiten.DrawImageOptions{}
			option.GeoM.Translate(float64(i*game.cellSize+game.boardMargin), float64(j*game.cellSize+game.boardMargin))
			if position&game.black > 0 {
				screen.DrawImage(blackImg, option)
				continue
			}
			if position&game.white > 0 {
				screen.DrawImage(whiteImg, option)
				continue
			}
		}
	}
}

func (game *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return game.boardSize + game.boardMargin*2, game.boardSize + game.boardMargin*2
}

func main() {
	game := &Game{
		cellSize:    50,
		boardMargin: 50,
		boardSize:   50 * 8,
		player:      YOU,
		black:       initialBlack,
		white:       initialWhite,
	}
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Hello world")
	if err := ebiten.RunGame(game); err != nil {
		panic("Error")
	}
}
