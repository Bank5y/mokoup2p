package main

import "errors"

// errElevationCancelled is returned when the user declines the UAC prompt.
var errElevationCancelled = errors.New("elevation cancelled")
