package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/blakesmith/ar"
	"github.com/dsnet/compress/bzip2"
	"github.com/fsnotify/fsnotify"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

func ExtractStanza(debfile string, filepath string, w io.Writer) error {
	slog.Debug("Loading metadata", "deb", debfile)

	f, err := os.Open(debfile)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	h1 := md5.New()
	h2 := sha1.New()
	h3 := sha256.New()
	tee := io.TeeReader(f, io.MultiWriter(h1, h2, h3))

	// Open the *.deb ar archive and search for control.{gz,xz,bz2,lzma}
	a := ar.NewReader(tee)
	var controlName string
	for {
		hdr, err := a.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		if strings.HasPrefix(hdr.Name, "control.tar") {
			controlName = hdr.Name
			break
		}
	}
	if controlName == "" {
		return fmt.Errorf("control file not found in %s", debfile)
	}

	// Select the correct decompression method for the file
	var cr io.Reader
	switch controlName {
	case "control.tar.gz":
		if cr, err = gzip.NewReader(a); err != nil {
			return err
		}
	case "control.tar.xz":
		if cr, err = xz.NewReader(a); err != nil {
			return err
		}
	case "control.tar.lzma":
		if cr, err = lzma.NewReader(a); err != nil {
			return err
		}
	case "control.tar.bz2":
		if cr, err = bzip2.NewReader(a, nil); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported control archive format: %s", controlName)
	}

	// Read the control file contents directly to the output writer
	tr := tar.NewReader(cr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		if hdr.Name == "./control" {
			if _, err := io.Copy(w, tr); err != nil {
				return err
			}
			break
		}
	}

	// Advance the reader to the end of the file
	if _, err = io.Copy(io.Discard, tee); err != nil {
		return err
	}

	filename := filepath
	if filepath == "" {
		filename = debfile
	}

	if _, err = w.Write([]byte("Filename: " + filename + "\nSize: " + fmt.Sprint(fi.Size()) + "\n")); err != nil {
		return err
	}

	// Print the checksums
	_, err = w.Write([]byte(
		"MD5sum: " + hex.EncodeToString(h1.Sum(nil)) + "\n" +
			"SHA1: " + hex.EncodeToString(h2.Sum(nil)) + "\n" +
			"SHA256: " + hex.EncodeToString(h3.Sum(nil)) + "\n\n",
	))

	return err
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "A self-contained debian package indexer and server.\n\n  Usage: %s [options] [folder]\n\n", os.Args[0])
		flag.PrintDefaults()
	}

	watch := flag.Bool("watch", false, "Enable watch mode")
	flag.BoolVar(watch, "w", false, "Enable watch mode (shorthand)")

	listen := flag.String("listen", "localhost:8080", "HTTP server listen location")
	flag.StringVar(listen, "l", "localhost:8080", "HTTP server listen location (shorthand)")

	silent := flag.Bool("silent", false, "Enable silent mode")
	flag.BoolVar(silent, "s", false, "Enable silent mode (shorthand)")

	verbose := flag.Bool("verbose", false, "Enable verbose mode")
	flag.BoolVar(verbose, "v", false, "Enable verbose mode (shorthand)")

	recursive := flag.Bool("recursive", false, "Search child directories")
	flag.BoolVar(recursive, "r", false, "Search child directories (shorthand)")

	flag.Parse()

	slog.SetLogLoggerLevel(slog.LevelInfo)
	if *verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	} else if *silent {
		slog.SetLogLoggerLevel(slog.LevelError)
	}

	maxdepth := 1
	if *recursive {
		maxdepth = 5000
	}

	folder := "."
	if flag.NArg() > 0 {
		folder = flag.Arg(0)
	}

	scan := func() error {
		start := time.Now()
		count, err := ScanAndWritePackages(folder, maxdepth)
		slog.Info(fmt.Sprintf("Indexed %d package(s) in %v", count, time.Since(start)))
		return err
	}

	if *watch {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Fatal(err)
		}

		defer watcher.Close()

		go func() {
			for {
				select {
				case ev, ok := <-watcher.Events:
					if !ok {
						return
					}

					f := path.Base(ev.Name)
					if f == "Packages" || f == "Packages.bz2" || f == "Packages.gz" {
						continue
					}

					if err = scan(); err != nil {
						log.Fatal(err)
					}

				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}

					log.Fatal(err)
				}
			}
		}()

		err = watcher.Add(folder)
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := scan(); err != nil {
		log.Fatal(err)
	}

	// Create a file server handler
	fs := http.FileServer(http.Dir(folder))

	// Create a custom handler function to log the requests
	handler := func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("Request", "method", r.Method, "path", r.URL.Path)
		fs.ServeHTTP(w, r)
	}

	slog.Info(fmt.Sprintf("Starting debserver on %s", *listen))
	log.Fatal(http.ListenAndServe(*listen, http.HandlerFunc(handler)))
}

func ScanAndWritePackages(folder string, maxdepth int) (int, error) {
	f, err := os.Create(path.Join(folder, "Packages"))
	if err != nil {
		return 0, err
	}
	gzf, err := os.Create(path.Join(folder, "Packages.gz"))
	if err != nil {
		return 0, err
	}
	bzf, err := os.Create(path.Join(folder, "Packages.bz2"))
	if err != nil {
		return 0, err
	}

	defer f.Close()
	defer gzf.Close()
	defer bzf.Close()

	gzw := gzip.NewWriter(gzf)
	bzw, err := bzip2.NewWriter(bzf, nil)
	if err != nil {
		return 0, err
	}

	defer gzw.Close()
	defer bzw.Close()

	w := io.MultiWriter(f, gzw, bzw)
	count, err := ScanPackages(folder, maxdepth, w)

	return count, err
}

// Scan and produce Packages file
func ScanPackages(folder string, maxdepth int, w io.Writer) (int, error) {
	count := 0

	err := filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if strings.Count(path, string(filepath.Separator)) > maxdepth {
				return fs.SkipDir
			}
		} else if filepath.Ext(path) == ".deb" {
			relpath, err := filepath.Rel(folder, path)
			if err != nil {
				return err
			}
			if err = ExtractStanza(path, "./"+relpath, w); err != nil {
				return err
			}
			count++
		}

		return nil
	})

	return count, err
}
