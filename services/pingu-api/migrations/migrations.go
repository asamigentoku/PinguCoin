// Package migrations は、pingu-api の DB スキーマのマイグレーション(番号付きの SQL ファイル)を、バイナリに埋め込む。
//
// ファイルの追加のしかた・ルールは pkg/dbmigrate と docs/VERSIONING.md を参照。要点:
//   - 変更は、次の番号の新しいファイル(NNNNNN_説明.sql)で足す。適用済みのファイルは、書き換えない。
//   - 前にしか進まない。間違えたときは、直す SQL を、新しいファイルで足す。
package migrations

import "embed"

// FS は、このフォルダの *.sql。
//
//go:embed *.sql
var FS embed.FS
