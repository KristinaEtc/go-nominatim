package fileproc

import (
	"bufio"
	"github.com/ventu-io/slf"
	"os"
)

const pwdCurr string = "Nominatim/lib/utils/fileprob"

// For writing to a file
func NewFileWriter(fileName string) (*FileWriter, error) {
	tmpFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	fs := FileWriter{File: tmpFile, Logger: slf.WithContext(pwdCurr)}
	defer fs.Logger.WithFields(slf.Fields{
		"file": fileName,
	}).Info("Created a new file writer.")

	return &fs, nil
}

type FileWriter struct {
	File   *os.File
	Writer *bufio.Writer
	Logger slf.StructuredLogger
}

func (f *FileWriter) GetWriter() *bufio.Writer {
	if f.Writer == nil {
		f.Writer = bufio.NewWriter(f.File)
		//f.Scanner.Split(bufio.ScanLines)
	}
	return f.Writer
}

// For using with in defer()
func (f *FileWriter) Close() error {
	f.Logger.Info("File writer is closing.")
	return f.File.Close()
}

// For reading from a file
// Functions are similar to Writer struct
func NewFileScanner(fileName string) (*FileScanner, error) {

	tmpFile, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	fs := FileScanner{File: tmpFile, Logger: slf.WithContext(pwdCurr)}

	defer fs.Logger.WithFields(slf.Fields{
		"file": fileName,
	}).Info("Created a new file scanner.")

	return &fs, nil
}

type FileScanner struct {
	File    *os.File
	Scanner *bufio.Scanner
	Logger  slf.StructuredLogger
}

func (f *FileScanner) GetScanner() *bufio.Scanner {
	if f.Scanner == nil {
		f.Scanner = bufio.NewScanner(f.File)
		//f.Scanner.Split(bufio.ScanLines)
	}
	return f.Scanner
}

func (f *FileScanner) Close() error {
	f.Logger.Info("File scanner is closing.")
	return f.File.Close()
}
