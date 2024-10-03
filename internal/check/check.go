package check

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zvdy/parsero-go/pkg/colors"
)

var pathlist []string

func ConnCheck(url string, only200 bool) {
	resp, err := http.Get("http://" + url + "/robots.txt")
	if err != nil {
		fmt.Println(colors.FAIL + "No robots.txt file has been found." + colors.ENDC)
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Disallow: /") {
			path := strings.TrimPrefix(line, "Disallow: /")
			pathlist = append(pathlist, strings.TrimSpace(path))
		}
	}

	client := &http.Client{}
	for _, p := range pathlist {
		disurl := "http://" + url + "/" + p
		req, _ := http.NewRequest("GET", disurl, nil)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			fmt.Println(colors.OKGREEN + disurl + " " + resp.Status + colors.ENDC)
		} else if !only200 {
			fmt.Println(colors.FAIL + disurl + " " + resp.Status + colors.ENDC)
		}
	}
}

func PrintDate(url string) {
	fmt.Println("Starting Parsero v0.81 (https://github.com/zvdy/parsero-go) at " + time.Now().Format("01/02/2006 15:04:05"))
	fmt.Println("Parsero scan report for " + url)
}
