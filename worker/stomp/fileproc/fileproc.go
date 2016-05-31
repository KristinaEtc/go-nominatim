package fileproc

import (
	"bufio"
	"os"
)

type FileScanner struct {
	File    *os.File
	Scanner *bufio.Scanner
	Reader  *bufio.Reader
}

func NewFileScanner(fileName string) (*FileScanner, error) {
	tmpFile, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	fs := FileScanner{File: tmpFile}
	return &fs, nil
}

func (f *FileScanner) Close() error {
	return f.File.Close()
}

func (f *FileScanner) GetScanner() *bufio.Scanner {
	if f.Scanner == nil {
		f.Scanner = bufio.NewScanner(f.File)
		//f.Scanner.Split(bufio.ScanLines)
	}
	return f.Scanner
}
