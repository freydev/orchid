package web

import "embed"

//go:embed index.html app.js vendor/*
var FS embed.FS
