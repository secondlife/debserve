# Debserve

**debserve** is a small, self-contained debian package indexer and server
designed to scratch a very specific itch: installing local debian files
into docker images. It may have other utility.

## Instructions

By default, **debserve** will scan for \*.deb files in the current directory
and host them at localhost:8080. Additional options:

```
A self-contained debian package indexer and server.

  Usage: ./debserve [options] [folder]

  -l string
        HTTP server listen location (shorthand) (default "localhost:8080")
  -listen string
        HTTP server listen location (default "localhost:8080")
  -r    Search child directories (shorthand)
  -recursive
        Search child directories
  -s    Enable silent mode (shorthand)
  -silent
        Enable silent mode
  -v    Enable verbose mode (shorthand)
  -verbose
        Enable verbose mode
  -w    Enable watch mode (shorthand)
  -watch
        Enable watch mode
```

## Docker image

A docker image, `lindenlab/debserve`, is also available, and can use a local volume mount
to index and serve local packages:

```sh
docker run -it --rm -v $(pwd):/packages -p 12321:80 lindenlab/debserve
```