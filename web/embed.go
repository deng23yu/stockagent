// Package web 内嵌前端构建产物 (web/dist)。
//
// dist 随仓库提交，保证 go install 用户开箱即有 Web UI;
// 前端变更后需重新构建: cd web && npm run build。
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
