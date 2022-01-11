package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	InfoLogger  *log.Logger
	DebugLogger *log.Logger
	ErrorLogger *log.Logger
)

func init() {
	logfile, err := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		log.Fatal(err)
	}
	InfoLogger = log.New(logfile, "INFO: ", log.Ldate|log.Ltime)
	ErrorLogger = log.New(logfile, "ERROR: ", log.Ldate|log.Ltime)
}

func format_string(format string, args ...interface{}) string {
	args2 := make([]string, len(args))
	for i, v := range args {
		if i%2 == 0 {
			args2[i] = fmt.Sprintf("{%v}", v)
		} else {
			args2[i] = fmt.Sprint(v)
		}
	}
	r := strings.NewReplacer(args2...)
	return r.Replace(format)
}

func Error(format string, args ...interface{}) {
	_, fn, line, _ := runtime.Caller(1)
	// below adds caller info to the string to be logged
	format = filepath.Base(fn) + ":" + strconv.Itoa(line) + ": " + format
	log_msg := format_string(format, args...)
	fmt.Println(log_msg)
	ErrorLogger.Println(log_msg)
}

func Info(format string, args ...interface{}) {
	_, fn, line, _ := runtime.Caller(1)
	// below adds caller info to the string to be logged
	format = filepath.Base(fn) + ":" + strconv.Itoa(line) + ": " + format
	log_msg := format_string(format, args...)
	fmt.Println(log_msg)
	InfoLogger.Println(log_msg)
}
