# lumberjack

Fork from "https://github.com/natefinch/lumberjack"

New Features:

- support rotate by date or houre

## use

```go
import "gopkg.in/huyighuan/lumberjack.v4"
```

### Lumberjack is a Go package for writing logs to rolling files.


Lumberjack is intended to be one part of a logging infrastructure.
It is not an all-in-one solution, but instead is a pluggable
component at the bottom of the logging stack that simply controls the files
to which logs are written.

Lumberjack plays well with any logging package that can write to an
io.Writer, including the standard library's log package.

Lumberjack assumes that only one process is writing to the output files.
Using the same lumberjack configuration from multiple processes on the same
machine will result in improper behavior.


**Example Rotate By Size**

To use lumberjack with the standard library's log package, just pass it into the SetOutput function when your application starts.

Code:

```go
	l, _ := lumberjack.NewRoller(
		"/var/log/myapp/foo.log",
		&lumberjack.Options{
			MaxBackups: 3,
			MaxAge:     28 * time.Hour * 24, // 28 days
			Compress:   true,
			MaxSize:    500 * 1024 * 1024,
		})
	log.SetOutput(l)
```


**Example Rotate By Date **

Code:

```go
	l, _ := lumberjack.NewRoller(
		"/var/log/myapp/foo.log",
		&lumberjack.Options{
		  	Filename:   "/var/log/myapp/foo.log",
			MaxAge:     28, //days
			RotateType: RotateDaily, //optioanl, RotateHourly or RotateDaily, If not set, use rotate by size
			RotateTime: 1, // optional, default 1
		})
	log.SetOutput(l)
```

**Example Rotate By Houre **

Code:

```go
	l, _ := lumberjack.NewRoller(
		"/var/log/myapp/foo.log",
		&lumberjack.Options{
		   Filename:   "/var/log/myapp/foo.log",
			MaxAge:     28, //days
			RotateType: RotateHourly, //optioanl, RotateHourly or RotateDaily, If not set, use rotate by size
			RotateTime: 5, // optional, default 1
		})
	log.SetOutput(l)
```


### type Option

``` go
type Options struct {

    // MaxSize is the maximum size in megabytes of the log file before it gets rotated. It defaults to 100 megabytes.
    // optional, only used when RotateType is RotateSize or not set
	MaxSize int64 `json:"maxsize" yaml:"maxsize"`
	// MaxAge is the maximum time to retain old log files based on the timestamp
	// encoded in their filename. The default is not to remove old log files
	// based on age.
	MaxAge time.Duration `json:"maxage" yaml:"maxage"`

	// MaxBackups is the maximum number of old log files to retain. The default
	// is to retain all old log files (though MaxAge may still cause them to get
	// deleted.)
	MaxBackups int `json:"maxbackups" yaml:"maxbackups"`

	// LocalTime determines if the time used for formatting the timestamps in
	// backup files is the computer's local time. The default is to use UTC
	// time.
	LocalTime bool `json:"localtime" yaml:"localtime"`

	// Compress determines if the rotated log files should be compressed
	// using gzip. The default is not to perform compression.
	Compress bool `json:"compress" yaml:"compress"`

    // RotateType: optional:  RotateHourly, RotateDaily, RotateSize, default RotateSize
	RotateType RotateType `json:"rotate_type" yaml:"rotate_type"`
	// if RotateType is RotateHourly, need make 24/RotateTime > 0
	// if RotateType is not empty, default RotateTime is 1
	// Example: RotateType is RotateHourly, RotateTime is 5, means rotate log file every 5 hours, rotate at 00:00 05:00 10:00 15:00 20:00 25:00
	//  RotateType is RotateDaily, RotateTime is 2, means rotate log file every 2 days,
	// rotate at xxxx-xx-01 00:00,  xxxx-xx-03 00:00
	// or rotate at xxxx-xx-02 00:00,  xxxx-xx-04 00:00
	RotateTime uint `json:"rotate_time" yaml:"rotate_time"`

	// hook for rotate process
	Hook Hook `json:"-" yaml:"-"`
}
```

### type Hook

```go
type Hook struct {
	// call after rotate complete
	AfterRotate func(filepath string)
}
```

Logger is an io.WriteCloser that writes to the specified filename.

Logger opens or creates the logfile on first Write.  If the file exists and
is less than MaxSize megabytes, lumberjack will open and append to that file.
If the file exists and its size is >= MaxSize megabytes, the file is renamed
by putting the current time in a timestamp in the name immediately before the
file's extension (or the end of the filename if there's no extension). A new
log file is then created using original filename.

Whenever a write would cause the current log file exceed MaxSize megabytes,
the current file is closed, renamed, and a new log file created with the
original name. Thus, the filename you give Logger is always the "current" log
file.

Backups use the log file name given to Logger, in the form `name-timestamp.ext`
where name is the filename without the extension, timestamp is the time at which
the log was rotated formatted with the time.Time format of
`20060102150405` and the extension is the original extension.  For
example, if your Logger.Filename is `/var/log/foo/server.log`, a backup created
at 6:30pm on Nov 11 2016 would use the filename
`/var/log/foo/server-20060102150405.log`

### Cleaning Up Old Log Files
Whenever a new logfile gets created, old log files may be deleted.  The most
recent files according to the encoded timestamp will be retained, up to a
number equal to MaxBackups (or all of them if MaxBackups is 0).  Any files
with an encoded timestamp older than MaxAge days are deleted, regardless of
MaxBackups.  Note that the time encoded in the timestamp is the rotation
time, which may differ from the last time that file was written to.

If MaxBackups and MaxAge are both 0, no old log files will be deleted.











### func (\*Logger) Close
``` go
func (l *Logger) Close() error
```
Close implements io.Closer, and closes the current logfile.



### func (\*Logger) Rotate
``` go
func (l *Logger) Rotate() error
```
Rotate causes Logger to close the existing log file and immediately create a
new one.  This is a helper function for applications that want to initiate
rotations outside of the normal rotation rules, such as in response to
SIGHUP.  After rotating, this initiates a cleanup of old log files according
to the normal rules.

**Example**

Example of how to rotate in response to SIGHUP.

Code:

```go
l, _ := lumberjack.NewRoller(
	"/var/log/myapp/foo.log",
	&lumberjack.Options{
		Filename:   "/var/log/myapp/foo.log",
		MaxAge:     28, //days
		RotateType: RotateHourly, //optioanl, RotateHourly or RotateDaily, If not set, use rotate by size
		RotateTime: 5, // optional, default 1
	})
log.SetOutput(l)
c := make(chan os.Signal, 1)
signal.Notify(c, syscall.SIGHUP)

go func() {
    for {
        <-c
        l.Close()
    }
}()
```

### func (\*Logger) Write
``` go
func (l *Logger) Write(p []byte) (n int, err error)
```
Write implements io.Writer.  If a write would cause the log file to be larger
than MaxSize, the file is closed, renamed to include a timestamp of the
current time, and a new log file is created using the original log file name.
If the length of the write is greater than MaxSize, an error is returned.

