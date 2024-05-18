package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/henderiw/store"
	"github.com/pkg/errors"
)

func Build(files map[string]string) (v1.Image, error) {
	// copy files to tarbal
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)
	for fileName, data := range files {
		buf := bytes.NewBufferString(data)
		hdr := &tar.Header{
			Name: fileName,
			Mode: int64(0o644),
			Size: int64(buf.Len()),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := io.Copy(tw, buf); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	// Build image layer from tarball.
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(tarBuf.Bytes())), nil
	})
	if err != nil {
		return nil, err
	}
	// Append layer to empty image.
	return mutate.AppendLayers(Image, layer)
}

func BuildTgz(files map[string]string) ([]byte, error) {
	// Create a buffer to store the tar data in memory
	tarBuffer := new(bytes.Buffer)

	// Create a gzip writer
	gzipWriter := gzip.NewWriter(tarBuffer)
	defer gzipWriter.Close()

	// Create a tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add files to the tar archive
	for fileName, data := range files {
		// Create a new tar header
		header := &tar.Header{
			Name: fileName,
			Mode: int64(0o644),
			Size: int64(len(data)),
		}

		// Write the header to the tar archive
		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, errors.Wrap(err, "cannot write header")
		}
		// Write the file content to the tar archive
		if _, err := tarWriter.Write([]byte(data)); err != nil {
			return nil, errors.Wrap(err, "cannot write file content")
		}
	}
	// Finish writing the tar archive
	if err := tarWriter.Close(); err != nil {
		return nil, errors.Wrap(err, "cannot close tar writer")
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, errors.Wrap(err, "cannot close gzip writer")
	}

	return tarBuffer.Bytes(), nil
}

func TgzReader(ctx context.Context, r io.Reader, data store.Storer[[]byte]) error {
	// Create a gzip reader
	gzipReader, err := gzip.NewReader(r)
	if err != nil {
		return errors.Wrap(err, "cannot create gzip reader")
	}
	defer gzipReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzipReader)

	// Iterate through the contents of the tar file
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break // End of archive
		}

		if err != nil {
			return errors.Wrap(err, "error reading tar header")
		}

		// Print the name of the file or directory
		switch header.Typeflag {
		case tar.TypeDir:
			fmt.Println("dir:", header.Name)
			/*
				if err := os.Mkdir(header.Name, 0755); err != nil {
					log.Fatalf("ExtractTarGz: Mkdir() failed: %s", err.Error())
				}
			*/
		case tar.TypeReg:
			// Create a buffer to hold the file content in memory
			fmt.Println("Extracting:", header.Name)
			fileContent := new(bytes.Buffer)
			/*
				if err != nil {
					log.Fatalf("ExtractTarGz: Create() failed: %s", err.Error())
				}
			*/
			if _, err := io.Copy(fileContent, tarReader); err != nil {
				log.Fatalf("ExtractTarGz: Copy() failed: %s", err.Error())
			}
			//fmt.Println("File Content:")
			//fmt.Println(fileContent.String())
			data.Create(ctx, store.ToKey(header.Name), fileContent.Bytes())

		default:
			fmt.Println("unknown:", header.Name, header.Typeflag)
			/*
				log.Fatalf(
					"ExtractTarGz: uknown type: %s in %s",
					header.Typeflag,
					header.Name)
			*/
		}
	}
	return nil
}

func ReadTgz(tgzData []byte) ([]byte, error) {
	// Create a reader for the in-memory .tgz data
	tgzReader := bytes.NewReader(tgzData)

	// Create a gzip reader
	gzipReader, err := gzip.NewReader(tgzReader)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create gzip reader")
	}
	defer gzipReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzipReader)

	// Iterate through the contents of the tar file
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break // End of archive
		}

		if err != nil {
			return nil, errors.Wrap(err, "error reading tar header")
		}

		// Print the name of the file or directory
		switch header.Typeflag {
		case tar.TypeDir:
			fmt.Println("dir:", header.Name)
			/*
				if err := os.Mkdir(header.Name, 0755); err != nil {
					log.Fatalf("ExtractTarGz: Mkdir() failed: %s", err.Error())
				}
			*/
		case tar.TypeReg:
			// Create a buffer to hold the file content in memory
			fmt.Println("Extracting:", header.Name)
			fileContent := new(bytes.Buffer)
			/*
				if err != nil {
					log.Fatalf("ExtractTarGz: Create() failed: %s", err.Error())
				}
			*/
			if _, err := io.Copy(fileContent, tarReader); err != nil {
				log.Fatalf("ExtractTarGz: Copy() failed: %s", err.Error())
			}
			//fmt.Println("File Content:")
			//fmt.Println(fileContent.String())

		default:
			fmt.Println("unknown:", header.Name, header.Typeflag)
			/*
				log.Fatalf(
					"ExtractTarGz: uknown type: %s in %s",
					header.Typeflag,
					header.Name)
			*/
		}

		// Check if it's a regular file
		/*
			if header.Typeflag == tar.TypeReg {
				// Create a buffer to hold the file content in memory
				fileContent := new(bytes.Buffer)

				// Copy the file content from the tar archive to the buffer
				_, err = io.Copy(fileContent, tarReader)
				if err != nil {
					return nil, errors.Wrap(err, "error copying file content")
				}
			}
		*/
	}
	return nil, nil
}

func UnzipTgz(ctx context.Context, basepath string, tgzData []byte, data store.Storer[[]byte]) error {
	// Create a reader for the in-memory .tgz data
	tgzReader := bytes.NewReader(tgzData)

	// Create a gzip reader
	gzipReader, err := gzip.NewReader(tgzReader)
	if err != nil {
		return errors.Wrap(err, "cannot create gzip reader")
	}
	defer gzipReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzipReader)

	// Iterate through the contents of the tar file
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return errors.Wrap(err, "error reading tar header")
		}
		switch header.Typeflag {
		case tar.TypeDir:
			fmt.Println("dir:", header.Name)
			// TBD need to make dir?
		case tar.TypeReg:
			// Create a buffer to hold the file content in memory
			fmt.Println("Extracting:", header.Name)
			fileContent := new(bytes.Buffer)
			/*
				if err != nil {
					log.Fatalf("ExtractTarGz: Create() failed: %s", err.Error())
				}
			*/
			if _, err := io.Copy(fileContent, tarReader); err != nil {
				log.Fatalf("ExtractTarGz: Copy() failed: %s", err.Error())
			}
			data.Create(ctx, store.ToKey(filepath.Join(basepath, header.Name)), fileContent.Bytes())
		default:
			return fmt.Errorf("unknown type: %s, %b", header.Name, header.Typeflag)
		}
	}
	return nil
}
