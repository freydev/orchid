package web

import "embed"

//go:embed index.html app.js styles.css favicon.svg vendor/*
var FS embed.FS
