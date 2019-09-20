package main

import (
	"bytes"
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
	"time"
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

func main() {
	start := time.Now()
	imgPath := os.Args[1]

	img, err := LoadImage(imgPath)
	if err != nil {
		panic(err)
	}

	palette := GetPalette(img)
	// has to be > 11

	for i, p := range palette {
		if i%4 == 0 && i != 0 {
			fmt.Print("\n")
		}
		fmt.Printf("\033[48;2;%d;%d;%dm%s\033[49m", p[0], p[1], p[2], ToHex(p))
	}

	NewXre, err := os.Create("/home/seungheonoh/.XreCustomColor")
	defer NewXre.Close()
	if err != nil {
		panic(err)
	}
	OriXre, err := ioutil.ReadFile("/home/seungheonoh/.Xresources")
	if err != nil {
		panic(err)
	}

	var XresourceData string
	var ColorScheme [10]string
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
		value := GetColor(palette, ColorTamplete[i])
		for _, val := range value {
			gotVal := true
			for _, item := range ColorScheme {
				if item == ToHex(val) {
					gotVal = false
				}
			}
			if gotVal {
				ColorScheme[i] = ToHex(val)
				break
			}
		}
	}
	fmt.Println("\n", ColorScheme)

	XresourceData += fmt.Sprintf("*.background: #%s\n", ColorScheme[0])
	XresourceData += fmt.Sprintf("*.foreground: #%s\n", ColorScheme[1])
	XresourceData += fmt.Sprintf("*.color0: #%s\n", ColorScheme[2])
	XresourceData += fmt.Sprintf("*.color1: #%s\n", ColorScheme[3])
	XresourceData += fmt.Sprintf("*.color2: #%s\n", ColorScheme[4])
	XresourceData += fmt.Sprintf("*.color3: #%s\n", ColorScheme[5])
	XresourceData += fmt.Sprintf("*.color4: #%s\n", ColorScheme[6])
	XresourceData += fmt.Sprintf("*.color5: #%s\n", ColorScheme[7])
	XresourceData += fmt.Sprintf("*.color6: #%s\n", ColorScheme[8])
	XresourceData += fmt.Sprintf("*.color7: #%s\n", ColorScheme[9])

	NewXre.Write(append([]byte(XresourceData), OriXre...))

	ReloadXresource := exec.Command("xrdb", "/home/seungheonoh/.XreCustomColor")

	err = ReloadXresource.Run()
	if err != nil {
		panic(err)
	}

	end := time.Now()
	fmt.Println(end.Sub(start))
}
