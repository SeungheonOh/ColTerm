package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func GetRange(c uint8) int {
	if 0 <= c && c < 85 {
		return 0
	} else if 85 <= c && c < 170 {
		return 1
	} else if 170 <= c && c <= 255 {
		return 2
	}
	return 3
}

func GetPalette(img image.Image) [][3]uint8 {
	var Area [27]int
	var AreaAvg [27][3]uint32

	for i := 0; i < img.Bounds().Max.X; i++ {
		for j := 0; j < img.Bounds().Max.Y; j++ {
			r, g, b, _ := img.At(i, j).RGBA()
			Area[GetRange(uint8(r/257))*9+GetRange(uint8(g/257))*3+GetRange(uint8(b/257))]++
			AreaAvg[GetRange(uint8(r/257))*9+GetRange(uint8(g/257))*3+GetRange(uint8(b/257))][0] += r / 257
			AreaAvg[GetRange(uint8(r/257))*9+GetRange(uint8(g/257))*3+GetRange(uint8(b/257))][1] += g / 257
			AreaAvg[GetRange(uint8(r/257))*9+GetRange(uint8(g/257))*3+GetRange(uint8(b/257))][2] += b / 257
		}
	}
	for i, _ := range Area {
		if Area[i] == 0 {
			continue
		}
		for c := 0; c < 3; c++ {
			AreaAvg[i][c] = AreaAvg[i][c] / uint32(Area[i])
		}
	}

	var ret [][3]uint8
	for i := 0; i < 27; i++ {
		if Area[i] == 0 {
			continue
		}
		ret = append(ret, [3]uint8{uint8(AreaAvg[i][0]), uint8(AreaAvg[i][1]), uint8(AreaAvg[i][2])})
	}
	return ret
}

func LoadImage(PathOrLink string) (image.Image, error) {
	var data io.Reader
	if _, err := os.Stat(PathOrLink); os.IsNotExist(err) {
		// if it's not a file
		response, err := http.Get(PathOrLink)
		if err != nil {
			return nil, err
		}
		body, _ := ioutil.ReadAll(response.Body)
		defer response.Body.Close()
		data = bytes.NewReader(body)
	} else {
		// if it's a file
		data, err = os.Open(PathOrLink)
		if err != nil {
			return nil, err
		}
	}
	img, _, err := image.Decode(data)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func ToHex(color [3]uint8) string {
	var ret string
	for _, c := range color {
		if c <= 16 {
			ret += "0"
		}
		ret += fmt.Sprintf("%X", c)
	}
	return ret
}

func GetColor(color [][3]uint8, sample [3]uint8) [][3]uint8 {
	sort.Slice(color, func(i, j int) bool {
		var c1, c2 float64
		for c := 0; c < 3; c++ {
			c1 += math.Abs(float64(sample[c]) - float64(color[i][c]))
			c2 += math.Abs(float64(sample[c]) - float64(color[j][c]))
		}
		return c1 < c2
	})
	return color
}

func NormalizeColor(color [3]uint8) [3]uint8 {
	min := uint8(255)
	for _, c := range color {
		if c < min {
			min = c
		}
	}
	for i, _ := range color {
		color[i] -= min
	}
	return color
}

func GetBG(color [3]uint8, brightness uint8) [3]uint8 {
	norm := NormalizeColor(color)
	for i, _ := range norm {
		if int(norm[i])+int(brightness) > 255 {
			norm[i] = 255
			continue
		} else if int(norm[i])+int(brightness) < 0 {
			norm[i] = 0
			continue
		}
		norm[i] += brightness
	}
	return norm
}

func main() {
	const (
		XresourceCache = "/.cache/colterm/Xresources"
	)

	FileIn := flag.String("f", "", "Input file, use this option or put file behind the options")

	BgBrightness := flag.Int("bg", 20, "Set background brightness(0 - 255)")
	FgBrightness := flag.Int("fg", 150, "Set foreground brightness(0 - 255)")

	FileExport := flag.String("e", "", "Export file to (Path)")
	Tamplate := flag.String("t", "", "Create color scheme with a given tamplate file, created file will saved in your home directory")
	PaletteOnly := flag.Bool("n", false, "Print colors only without applying to Xresources")

	flag.Parse()

	if len(flag.Args()) == 0 && *FileIn == "" {
		flag.PrintDefaults()
		return
	}

	imgPath := *FileIn

	if imgPath == "" {
		imgPath = flag.Args()[0]
	}

	img, err := LoadImage(imgPath)
	if err != nil {
		panic(err)
	}

	Palette := GetPalette(img)
	if len(Palette) < 10 {
		panic("This image is not appropriate for generating color scheme")
	}

	var ColorScheme [10][3]uint8
	var ColorTamplete = [10][3]uint8{
		[3]uint8{0, 0, 0},
		[3]uint8{150, 150, 150},
		[3]uint8{0, 0, 0},
		[3]uint8{255, 0, 0},
		[3]uint8{0, 255, 0},
		[3]uint8{255, 255, 0},
		[3]uint8{0, 0, 150},
		[3]uint8{255, 0, 255},
		[3]uint8{0, 255, 255},
		[3]uint8{255, 255, 255},
	}

	for i := 0; i < 10; i++ {
		value := GetColor(Palette, ColorTamplete[i])
		for _, val := range value {
			gotVal := true
			for _, item := range ColorScheme {
				if item == val {
					gotVal = false
				}
			}
			if gotVal {
				ColorScheme[i] = val
				break
			}
		}
	}
	if *BgBrightness != -1 {
		ColorScheme[0] = GetBG(GetColor(Palette, ColorTamplete[0])[0], uint8(*BgBrightness))
	}
	if *FgBrightness != -1 {
		ColorScheme[1] = GetBG(GetColor(Palette, ColorTamplete[1])[0], uint8(*FgBrightness))
	}

	fmt.Printf("   ¯\\_(•_•)_/¯   \nHeres your colors\n\n")
	fmt.Printf("Background \033[48;2;%d;%d;%dm%s\033[49m\n", ColorScheme[0][0], ColorScheme[0][1], ColorScheme[0][2], ToHex(ColorScheme[0]))
	fmt.Printf("Foreground \033[48;2;%d;%d;%dm%s\033[49m\n", ColorScheme[1][0], ColorScheme[1][1], ColorScheme[1][2], ToHex(ColorScheme[1]))
	for i := 2; i < len(ColorScheme); i++ {
		c := ColorScheme[i]
		fmt.Printf("Color%d     \033[48;2;%d;%d;%dm%s\033[49m\n", i-2, c[0], c[1], c[2], ToHex(c))
	}

	if *PaletteOnly {
		return
	}

	if *Tamplate != "" {
		StTamplate, err := ioutil.ReadFile(*Tamplate)
		if err != nil {
			panic(err)
		}
		StData := string(StTamplate)
		StExport, err := os.Create(os.Getenv("HOME") + "/Colterm-" + *Tamplate)
		defer StExport.Close()
		if err != nil {
			panic(err)
		}

		StData = strings.ReplaceAll(StData, "background", "#"+ToHex(ColorScheme[0]))
		StData = strings.ReplaceAll(StData, "foreground", "#"+ToHex(ColorScheme[1]))
		StData = strings.ReplaceAll(StData, "cursor", "#"+ToHex(ColorScheme[1]))
		for i := 0; i < len(ColorScheme)-2; i++ {
			StData = strings.ReplaceAll(StData, fmt.Sprintf("color%d", i), "#"+ToHex(ColorScheme[i+2]))
			StData = strings.ReplaceAll(StData, fmt.Sprintf("color%d", i+8), "#"+ToHex(ColorScheme[i+2]))
		}

		_, err = StExport.Write([]byte(StData))
		StExport.Close()
		if err != nil {
			panic(err)
		}
	}

	// setting Xresources
	var XresourcePath string
	if *FileExport == "" {
		XresourcePath = os.Getenv("HOME") + XresourceCache
	} else {
		XresourcePath = *FileExport + "/ColTerm-Xresource"
	}
	NewXre, err := os.Create(XresourcePath)
	defer NewXre.Close()
	if err != nil {
		panic(err)
	}

	var XresourceData string
	XresourceData += fmt.Sprintf("! Created by Colterm with %s\n", imgPath)
	XresourceData += fmt.Sprintf("*.background: #%s\n", ToHex(ColorScheme[0]))
	XresourceData += fmt.Sprintf("*.foreground: #%s\n", ToHex(ColorScheme[1]))
	for i := 0; i < 8; i++ {
		XresourceData += fmt.Sprintf("*.color%d: #%s\n", i, ToHex(ColorScheme[i+2]))
	}

	NewXre.Write([]byte(XresourceData))

	ReloadXresource := exec.Command("xrdb", "-merge", "-quiet", os.Getenv("HOME")+XresourceCache)

	err = ReloadXresource.Run()
	if err != nil {
		panic(err)
	}

}
