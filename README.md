# Parsero

[![Go Report Card](https://goreportcard.com/badge/github.com/zvdy/parsero-go)](https://goreportcard.com/report/github.com/zvdy/parsero-go)
[![AUR](https://img.shields.io/aur/version/parsero-go?logo=archlinux&logoColor=white)](https://aur.archlinux.org/packages/parsero-go)

<img src="https://i.imgur.com/INJgn0i.png" alt="parseropher" width="400">

Parsero is a fast, concurrent Golang tool that reads the Robots.txt
file of a web server and analyzes the Disallow entries. The Disallow
entries tell the search engines what directories or files hosted on a
web server mustn't be indexed. For example, "Disallow: /portal/login"
means that the content on www.hackthissite.org/portal/login is not allowed
to be indexed by crawlers like Google, Bing, Yahoo... This is the way
administrators avoid sharing sensitive or private information
with search engines.

But sometimes these paths listed in the Disallow entries are directly
accessible by users without using a search engine, just by visiting
the URL and the Path, and sometimes they are not available to be visited
by anybody. Because it is really common that administrators write
numerous Disallow entries where some are available and some are
not, you can use Parsero to check the HTTP status code of each
Disallow entry to automatically determine if these directories are
available or not.

Also, the fact an administrator writes a robots.txt, doesn't mean
that the files or directories typed in the Disallow entries will not
be indexed by search engines. For this reason, Parsero is
capable of searching in Bing to locate content indexed without the web
administrator authorization. Parsero will check the HTTP status code in
the same way for each search result.

When you execute Parsero, you can see the HTTP status codes. For example:

    200 OK          The request has succeeded.
    403 Forbidden   The server understood the request, but is refusing to fulfill it.
    404 Not Found   The server hasn't found anything matching the Request-URI.
    302 Found       The requested resource resides temporarily under a different URI.
    ...

## Installation
To install Parsero, you need to have Golang installed on your machine. You can download and install Golang from [here](https://golang.org/dl/).

Once you have Golang installed, you can install Parsero by running the following command:

```sh
go get -u github.com/zvdy/parsero-go
```

## Arch installation
Parsero is available in the Arch User Repository (AUR). You can install it using any AUR helper like [yay](https://github.com/Jguer/yay) or [paru](https://github.com/Morganamilo/paru)

```sh
# Using yay
yay -S parsero-go
AUR Explicit (1): parsero-go-2.0.0-1
:: PKGBUILD up to date, skipping download: parsero-go
  1 parsero-go                       (Installed) (Build Files Exist)
==> Packages to cleanBuild?
==> [N]one [A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)

# Using paru
paru -S parsero-go
:: Resolving dependencies...
:: Calculating conflicts...
:: Calculating inner conflicts...

Aur (1)         Old Version   New Version    Make Only
aur/parsero-go  r5.17b8c11-1  2.0.0-1        No

:: Proceed with installation? [Y/n]: 
```

## Usage
You can run Parsero using the following command:

```sh
parsero-go --url <URL> [options]
```

### Options:
- `--url value`: Type the URL which will be analyzed.
- `--only200`: Show only the 'HTTP 200' status code.
- `--file value`: Scan a list of domains from a list.
- `--search-disallow`, `--sb`: Search for disallowed entries using Bing (optional).
- `--concurrency value`, `-c value`: Number of concurrent workers (default: number of CPU cores).
- `--json value`, `-j value`: Export results to JSON file (specify filename).
- `--json-stdout`: Print JSON results to stdout instead of normal output.
- `--help`, `-h`: Show help.

### Examples:

Basic scanning of a website's robots.txt:
```sh
parsero-go --url http://hackthissite.org
```

Show only pages with 200 status code:
```sh
parsero-go --url http://hackthissite.org --only200
```

For faster processing on large websites, use the concurrency flag:
```sh
parsero-go --url http://hackthissite.org --concurrency 32
```

Search for indexed Disallow entries in Bing:
```sh
parsero-go --url http://hackthissite.org --search-disallow
```

Export results to JSON file:
```sh
parsero-go --url http://hackthissite.org --json results.json
```

Output in JSON format to stdout (useful for piping to other tools):
```sh
parsero-go --url http://hackthissite.org --json-stdout | jq
```

Process multiple domains from a file:
```sh
parsero-go --file domains.txt --only200
```

## Terminal Session Example

You can watch a recorded terminal session here:

[![asciinema recording](https://asciinema.org/a/Vd0kE9zVyPPwqLNjsEGDr4IZg.png)](https://asciinema.org/a/Vd0kE9zVyPPwqLNjsEGDr4IZg)

## Performance

Parsero uses worker pools to process Disallow entries concurrently, which significantly improves performance when analyzing websites with large robots.txt files. By default, Parsero uses a number of workers equal to the available CPU cores, but you can adjust this with the `--concurrency` flag.

For searching operations with Bing, half the number of available CPU cores is used to avoid rate limiting from search engines.

The tool has been optimized to:
- Use HEAD requests before falling back to GET requests
- Implement appropriate timeouts to avoid hanging on slow resources
- Utilize connection pooling for better network throughput
- Properly close connections to avoid resource leaks

## JSON Export

Parsero can export its results in JSON format, either to a file using the `--json` flag or to stdout using the `--json-stdout` flag. This makes it easy to integrate Parsero with other tools in your workflow.

The JSON output includes:
- Timestamp of the scan
- Target URL
- Duration of the scan in seconds
- All results with their URLs, status codes, and error messages (if any)
- Statistics about total paths, success codes (200), other status codes, and errors

When using the `--only200` flag, the JSON output will only include results with a 200 status code.

## 

## Docker Setup

You can use the [Dockerfile](Dockerfile) in the root of the repository to build parsero as a container.

### Step-by-Step Docker Setup

1. **Build the Docker Image**:
   Open a terminal in the root of your project directory and run the following command to build the Docker image:

   ```sh
   docker build -t parsero:latest .
   ```

2. **Run the Docker Container**:
   After building the image, you can run it using the following command:

   ```sh
   docker run -it --rm parsero:latest --url http://hackthissite.org --only200
   ```

### Docker Hub

You can retrieve the image directly from Docker hub too.

   ```sh
   docker pull zvdy/parsero:latest
   docker run -it --rm zvdy/parsero:latest --url http://hackthissite.org --only200
   ```

## License
This project is licensed under the MIT. See the [LICENSE](LICENSE) file for details.

## Contributing
Contributions are welcome! Please open an issue or submit a pull request for any changes.

### Acknowledgements
This project is a port of [parsero](https://github.com/behindthefirewalls/Parsero) which is written in Python, the repository ports it to Golang in order to optimize the speed in long `robots.txt` files as well as adds new functionalities.
