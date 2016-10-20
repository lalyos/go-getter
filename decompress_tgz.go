package getter

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// TarGzipDecompressor is an implementation of Decompressor that can
// decompress tar.gzip files.
type TarGzipDecompressor struct{}

func (d *TarGzipDecompressor) Decompress(dst, src string, dir bool) error {
	// If we're going into a directory we should make that first
	mkdir := dst
	if !dir {
		mkdir = filepath.Dir(dst)
	}
	if err := os.MkdirAll(mkdir, 0755); err != nil {
		return err
	}

	// File first
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	// Gzip compression is second
	gzipR, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("Error opening a gzip reader for %s: %s", src, err)
	}
	defer gzipR.Close()

	// Once gzip decompressed we have a tar format
	tarR := tar.NewReader(gzipR)
	done := false
	for {
		hdr, err := tarR.Next()
		if err == io.EOF {
			if !done {
				// Empty archive
				return fmt.Errorf("empty archive: %s", src)
			}

			return nil
		}
		if err != nil {
			return err
		}

		path := dst
		if dir {
			path = filepath.Join(path, hdr.Name)
		}

		if hdr.FileInfo().IsDir() {
			if !dir {
				return fmt.Errorf("expected a single file: %s", src)
			}

			// A directory, just make the directory and continue unarchiving...
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}

			continue
		}

		// We have a file. If we already decoded, then it is an error
		if !dir && done {
			return fmt.Errorf("expected a single file, got multiple: %s", src)
		}

		// Mark that we're done so future in single file mode errors
		done = true

		if (hdr.Typeflag == tar.TypeSymlink) {
			// If the type is a symlink we re-write it and
			// continue instead of attempting to copy the contents
			if err := os.Symlink(hdr.Linkname, path); err != nil {
				return fmt.Errorf("failed writing symbolic link: %s", err)
			}

			continue
		}

		if (hdr.Typeflag == tar.TypeLink) {
			// If the type is a link ("hardlink") we re-write it and
			// continue instead of attempting to copy the contents
			if err := os.Link(hdr.Linkname, path); err != nil {
				return fmt.Errorf("failed writing link: %s", err)
			}

			continue
		}

		// Open the file for writing
		dstF, err := os.Create(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(dstF, tarR)
		dstF.Close()
		if err != nil {
			return err
		}

		// Chmod the file
		if err := os.Chmod(path, hdr.FileInfo().Mode()); err != nil {
			return err
		}
	}
}
