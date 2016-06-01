package fileproc

import (
	"bufio"
	"os"
)

type FileScanner struct {
	File    *os.File
	Scanner *bufio.Scanner
}

type FileWriter struct {
	File   *os.File
	Writer *bufio.Writer
}

func NewFileWriter(fileName string) (*FileWriter, error) {
	tmpFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0660)
	if err != nil {
		return nil, err
	}
	fs := FileWriter{File: tmpFile}
	return &fs, nil
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

func (f *FileWriter) Close() error {
	return f.File.Close()
}

func (f *FileScanner) GetScanner() *bufio.Scanner {
	if f.Scanner == nil {
		f.Scanner = bufio.NewScanner(f.File)
		//f.Scanner.Split(bufio.ScanLines)
	}
	return f.Scanner
}

func (f *FileWriter) GetWriter() *bufio.Writer {
	if f.Writer == nil {
		f.Writer = bufio.NewWriter(f.File)
		//f.Scanner.Split(bufio.ScanLines)
	}
	return f.Writer
}
