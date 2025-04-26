# Parsero

[![Go Report Card](https://goreportcard.com/badge/github.com/zvdy/parsero-go)](https://goreportcard.com/report/github.com/zvdy/parsero-go)

![parseropher](https://i.imgur.com/INJgn0i.png)

Parsero is a free script written in Golang which reads the Robots.txt
file of a web server and looks at the Disallow entries. The Disallow
entries tell the search engines what directories or files hosted on a
web server mustn't be indexed. For example, "Disallow: /portal/login"
means that the content on www.example.com/portal/login it's not allowed
to be indexed by crawlers like Google, Bing, Yahoo... This is the way
the administrator have to not share sensitive or private information
with the search engines.

But sometimes these paths typed in the Disallows entries are directly
accessible by the users without using a search engine, just visiting
the URL and the Path, and sometimes they are not available to be visited
by anybody. Because it is really common that the administrators write
a lot of Disallows and some of them are available and some of them are
not, you can use Parsero in order to check the HTTP status code of each
Disallow entry in order to check automatically if these directories are
available or not.

Also, the fact the administrator write a robots.txt, it doesn't mean
that the files or directories typed in the Disallow entries will not
be indexed by Bing, Google, Yahoo, etc. For this reason, Parsero is
capable of searching in Bing to locate content indexed without the web
administrator authorization. Parsero will check the HTTP status code in
the same way for each Bing result.

When you execute Parsero, you can see the HTTP status codes. For example,
the codes bellow:

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

## Usage
You can run Parsero using the following command:

```sh
parsero --url <URL> [--only200] [--search] [--engine <ENGINE>] [--file <FILE>] [--concurrency <N>]
```

Options:
- `--url`: Type the URL which will be analyzed.
- `--only200`: Show only the 'HTTP 200' status code.
- `--search`: Search for indexed Disallow entries.
- `--engine`: Search engine to use (supported: bing, google).
- `--file`: Scan a list of domains from a list.
- `--concurrency` or `-c`: Number of concurrent workers (default: number of CPU cores).

Example:
```sh
parsero --url http://example.com --only200
```

For faster processing on large websites, use the concurrency flag:
```sh
parsero --url http://example.com --concurrency 16
```

To search indexed Disallow entries using Google:
```sh
parsero --url http://example.com --search --engine google
```

To search indexed Disallow entries using Bing (default):
```sh
parsero --url http://example.com --search --engine bing
```

## Terminal Session Example

You can watch a recorded terminal session here:

[![asciinema recording](https://asciinema.org/a/Vd0kE9zVyPPwqLNjsEGDr4IZg.png)](https://asciinema.org/a/Vd0kE9zVyPPwqLNjsEGDr4IZg)

## Performance

Parsero uses worker pools to process Disallow entries concurrently, which significantly improves performance when analyzing websites with large robots.txt files. By default, Parsero uses a number of workers equal to the available CPU cores, but you can adjust this with the `--concurrency` flag.

Benchmark results show significant performance improvements with concurrency. For example, processing bing.com with 16 concurrent workers is much faster than sequential processing.

## Docker Setup

You can use the [Dockerfile](Dockerfile) in the root of the repository in order to build parsero as a container. 

### Step-by-Step Docker Setup


1. **Build the Docker Image**:
   Open a terminal in the root of your project directory and run the following command to build the Docker image:

   ```sh
   docker build -t parsero:latest .
   ```

2. **Run the Docker Container**:
   After building the image, you can run it using the following command:

   ```sh
   docker run -it --rm parsero:latest --url http://example.com --only200
   ```

### Docker Hub

You can retrieve the image directly from Docker hub too.

   ```sh
   docker pull zvdy/parsero:latest
   docker run -it --rm zvdy/parsero:latest --url http://example.com --only200
   ```

## License
This project is licensed under the MIT. See the [LICENSE](LICENSE) file for details.

## Contributing
Contributions are welcome! Please open an issue or submit a pull request for any changes.

### Acknowledgements
This project is a port of [parsero](https://github.com/behindthefirewalls/Parsero) which is written in Python, the repository ports it to Golang in order to optimize the speed in long `robots.txt` files.
