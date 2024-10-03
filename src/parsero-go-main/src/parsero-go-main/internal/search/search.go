// internal/search/search.go
package search

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/zvdy/parsero-go/pkg/colors"
)

var pathlist []string

func SearchBing(url string, only200 bool) {
	fmt.Println("\nSearching the Disallows entries in Bing...\n")
	client := &http.Client{}
	for _, p := range pathlist {
		disurl := "http://" + url + "/" + p
		searchURL := "http://www.bing.com/search?q=site:" + disurl
		req, _ := http.NewRequest("GET", searchURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			continue
		}

		doc.Find("cite").Each(func(i int, s *goquery.Selection) {
			citeText := s.Text()
			if strings.Contains(citeText, url) {
				req, _ := http.NewRequest("GET", citeText, nil)
				resp, err := client.Do(req)
				if err != nil {
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode == 200 {
					fmt.Println(colors.OKGREEN + " - " + citeText + " " + resp.Status + colors.ENDC)
				} else if !only200 {
					fmt.Println(colors.FAIL + " - " + citeText + " " + resp.Status + colors.ENDC)
				}
			}
		})
	}
}
