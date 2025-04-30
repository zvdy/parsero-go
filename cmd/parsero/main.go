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
	"github.com/zvdy/parsero-go/pkg/export"
	"github.com/zvdy/parsero-go/pkg/types"
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
			&cli.StringFlag{
				Name:  "file",
				Usage: "Scan a list of domains from a list",
			},
			&cli.BoolFlag{
				Name:    "search-disallow",
				Aliases: []string{"sb"},
				Usage:   "Search for disallowed entries using Bing (optional)",
			},
			&cli.IntFlag{
				Name:    "concurrency",
				Aliases: []string{"c"},
				Usage:   "Number of concurrent workers (default: number of CPU cores)",
				Value:   runtime.NumCPU(),
			},
			&cli.StringFlag{
				Name:    "json",
				Aliases: []string{"j"},
				Usage:   "Export results to JSON file (specify filename)",
			},
			&cli.BoolFlag{
				Name:  "json-stdout",
				Usage: "Print JSON results to stdout instead of normal output",
			},
		},
		Action: func(c *cli.Context) error {
			url := c.String("url")
			only200 := c.Bool("only200")
			file := c.String("file")
			searchDisallow := c.Bool("search-disallow")
			concurrency := c.Int("concurrency")
			jsonFile := c.String("json")
			jsonStdout := c.Bool("json-stdout")

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

			if !jsonStdout {
				logo.PrintLogo()
			}

			for _, url := range urls {
				url = strings.TrimPrefix(url, "http://")
				startTime := time.Now()

				if !jsonStdout {
					check.PrintDate(url)
				}

				// Check disallowed URLs in robots.txt
				checkResults := check.ConnCheck(url, only200, concurrency)

				// Initialize searchResults
				var searchResults []types.Result

				// Only search for disallowed entries if the flag is set
				if searchDisallow {
					if !jsonStdout {
						fmt.Println("\nSearching for disallowed entries using Bing...")
					}
					searchResults = search.SearchDisallowEntries(url, only200, concurrency)
				}

				// Combine all results for export
				var allResults []types.Result
				allResults = append(allResults, checkResults...)
				allResults = append(allResults, searchResults...)

				duration := time.Since(startTime)

				// Handle JSON output if requested
				if jsonFile != "" || jsonStdout {
					scanResult := export.CreateScanResult(url, duration, allResults, only200)

					if jsonStdout {
						jsonStr, err := export.ToJSON(scanResult)
						if err != nil {
							fmt.Println(colors.FAIL + "Error creating JSON output: " + err.Error() + colors.ENDC)
						} else {
							fmt.Println(jsonStr)
						}
					}

					if jsonFile != "" {
						err := export.SaveToFile(scanResult, jsonFile)
						if err != nil {
							fmt.Println(colors.FAIL + "Error saving JSON to file: " + err.Error() + colors.ENDC)
						} else if !jsonStdout {
							fmt.Println(colors.OKGREEN + "Results exported to " + jsonFile + colors.ENDC)
						}
					}
				}

				if !jsonStdout {
					fmt.Printf("\nFinished in %.2f seconds.\n", duration.Seconds())
				}
			}
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}
