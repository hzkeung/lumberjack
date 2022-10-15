package lumberjack

import "time"

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

	Hook *Hook `json:"-" yaml:"-"`
}
