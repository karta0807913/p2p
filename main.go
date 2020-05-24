package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/akamensky/argparse"
	"github.com/schollz/progressbar/v3"
)

func main() {
	parser := argparse.NewParser("p2p", "A Simple Local Network Tool")
	code := parser.String("c", "code", &argparse.Options{
		Help:     "需要取得檔案的編號",
		Required: true,
	})

	save_path := parser.String("o", "output_path", &argparse.Options{
		Help:     "儲存路徑",
		Required: false,
		Default:  "./output",
	})

	shared_path := parser.String("s", "share_path", &argparse.Options{
		Help:     "分享檔案給別人",
		Required: false,
		Default:  "",
	})

	err := parser.Parse(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	var shared_file *os.File = nil
	if *shared_path != "" {
		shared_file, err = os.OpenFile(*shared_path, os.O_RDONLY, 0644)
		if err != nil {
			log.Fatalf("shared file error, %s\n", err)
		}
	}

	search_manager, err := CreateSearchManager()
	if err != nil {
		log.Fatalf("create manager failed error, %s\n", err)
	}
	if shared_file == nil {
		err := search_manager.SearchFile(*code)
		if err != nil {
			log.Fatalf("create manager failed error, %s\n", err)
		}
		search_manager.SearchStart(2 * time.Second)
		for pkg, ok := <-search_manager.PacketChannel; ; pkg, ok = <-search_manager.PacketChannel {
			search_manager.SearchStop()
			var bar *progressbar.ProgressBar
			if !ok {
				continue
			}
			rander_count := 0
			var err error
			response := pkg.Data.(*FileResponse)
			shared_file, err = search_manager.DownloadFile(response, pkg.IP.IP.String(), *save_path, func(delta, max_size int) {
				if bar == nil {
					bar = progressbar.NewOptions(max_size,
						progressbar.OptionEnableColorCodes(false),
						progressbar.OptionShowBytes(true),
						progressbar.OptionSetWidth(15),
						progressbar.OptionSetTheme(progressbar.Theme{
							Saucer:        "=",
							SaucerHead:    ">",
							SaucerPadding: "-",
							BarStart:      "[",
							BarEnd:        "]",
						}),
					)
				}
				bar.Add(delta)
				rander_count += 1
				if rander_count%50 == 0 {
					bar.RenderBlank()
					rander_count = 0
				}
			})
			fmt.Println()
			if err != nil {
				log.Printf("download file failed, error %s\n", err)
				search_manager.SearchStart(2 * time.Second)
				continue
			} else {
				break
			}
		}
		search_manager.SearchStop()
	}
	err = search_manager.ShareFile(*code, shared_file)
	search_manager.ShareStart()
	if err != nil {
		log.Fatalf("Share file error %s\n", err)
	}
	log.Printf("Share file %s \n", shared_file.Name())
	for pkg, ok := <-search_manager.PacketChannel; ok; pkg, ok = <-search_manager.PacketChannel {
		response := pkg.Data.(*FileResponse)
		log.Printf("found file %s, code %s\n", response.FileName, response.Code)
	}
}
