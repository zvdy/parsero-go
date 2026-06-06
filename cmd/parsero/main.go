package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/zvdy/parsero-go/internal/logo"
	"github.com/zvdy/parsero-go/internal/scanner"
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

				scn := bufio.NewScanner(f)
				for scn.Scan() {
					urls = append(urls, scn.Text())
				}
			}

			if url != "" {
				urls = append(urls, url)
			}

			if !jsonStdout {
				logo.PrintLogo()
			}

			for _, u := range urls {
				u = strings.TrimPrefix(u, "http://")
				startTime := time.Now()

				if !jsonStdout {
					printDate(u)
				}

				sc := scanner.New(nil, scanner.Options{
					Only200:     only200,
					SearchBing:  searchDisallow,
					Concurrency: concurrency,
				})

				results, disallow, err := sc.Run(context.Background(), u)
				if err != nil {
					if !jsonStdout {
						fmt.Println(colors.FAIL + err.Error() + colors.ENDC)
					}
				} else if len(disallow) == 0 {
					if !jsonStdout {
						fmt.Println(colors.YELLOW + "No Disallow entries found in robots.txt." + colors.ENDC)
					}
				} else if !jsonStdout {
					fmt.Printf("Found %d Disallow entries. Processing with %d workers...\n", len(disallow), concurrency)
					printResults(results, only200)
				}

				duration := time.Since(startTime)

				if jsonFile != "" || jsonStdout {
					scanResult := export.CreateScanResult(u, duration, results, only200)

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

// printResults keeps the original CLI output: 200s green, others red unless
// only200, errors skipped.
func printResults(results []types.Result, only200 bool) {
	for _, r := range results {
		if r.Error != nil {
			continue
		}
		prefix := ""
		if r.Source == scanner.SourceBing {
			prefix = " - "
		}
		if r.StatusCode == 200 {
			fmt.Println(colors.OKGREEN + prefix + r.URL + " " + r.Status + colors.ENDC)
		} else if !only200 {
			fmt.Println(colors.FAIL + prefix + r.URL + " " + r.Status + colors.ENDC)
		}
	}
}

func printDate(url string) {
	fmt.Println("Starting Parsero v2.0.0 (https://github.com/zvdy/parsero-go) at " + time.Now().Format("01/02/2006 15:04:05"))
	fmt.Println("Parsero scan report for " + url)
}
