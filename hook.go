package lumberjack

type Hook struct {
	// call after rotate complete
	AfterRotate func(filepath string)
}
