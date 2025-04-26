package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/zvdy/parsero-go/internal/check"
	"github.com/zvdy/parsero-go/internal/logo"
	"github.com/zvdy/parsero-go/internal/search"
	"github.com/zvdy/parsero-go/pkg/colors"
)

func main() {
	app := &cli.App{
		Name:  "parsero",
		Usage: "A Go based Robots.txt audit tool",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "url",
				Usage: "Type the URL which will be analyzed",
			},
			&cli.BoolFlag{
				Name:  "only200",
				Usage: "Show only the 'HTTP 200' status code",
			},
			&cli.BoolFlag{
				Name:  "search",
				Usage: "Search for indexed Disallow entries",
			},
			&cli.StringFlag{
				Name:  "engine",
				Usage: "Search engine to use (supported: bing, google)",
				Value: "bing",
			},
			&cli.StringFlag{
				Name:  "file",
				Usage: "Scan a list of domains from a list",
			},
			&cli.IntFlag{
				Name:    "concurrency",
				Aliases: []string{"c"},
				Usage:   "Number of concurrent workers (default: number of CPU cores)",
				Value:   runtime.NumCPU(),
			},
		},
		Action: func(c *cli.Context) error {
			url := c.String("url")
			only200 := c.Bool("only200")
			doSearch := c.Bool("search")
			engine := c.String("engine")
			file := c.String("file")
			concurrency := c.Int("concurrency")

			if url == "" && file == "" {
				logo.PrintLogo()
				cli.ShowAppHelp(c)
				return nil
			}

			var urls []string
			if file != "" {
				f, err := os.Open(file)
				if err != nil {
					logo.PrintLogo()
					fmt.Println(colors.FAIL + "[-] The file '" + file + "' doesn't exist." + colors.ENDC)
					return nil
				}
				defer f.Close()

				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					urls = append(urls, scanner.Text())
				}
			}

			if url != "" {
				urls = append(urls, url)
			}

			// Parse the search engine using our helper function
			searchEngine := search.ParseSearchEngine(engine)

			logo.PrintLogo()
			for _, url := range urls {
				if strings.HasPrefix(url, "http://") {
					url = strings.TrimPrefix(url, "http://")
				}
				startTime := time.Now()
				check.PrintDate(url)
				check.ConnCheck(url, only200, concurrency)
				if doSearch {
					search.SearchDisallowEntries(url, only200, concurrency, searchEngine)
				}
				fmt.Printf("\nFinished in %.2f seconds.\n", time.Since(startTime).Seconds())
			}
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}
