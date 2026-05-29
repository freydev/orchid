package web

import "embed"

//go:embed index.html app.js styles.css vendor/*
var FS embed.FS
