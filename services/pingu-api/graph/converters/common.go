// Package converters は、DTO を別の DTO に変換する「マッパー」を集める。
//
// pingu-api には、データの形だけを表す型(DTO)が2種類ある。
//   - orcanpb.*  : orcan-api との gRPC のやり取りに使う型(protobuf から生成)
//   - graph/model: GraphQL のレスポンスとしてクライアントに返す型(gqlgen が生成)
//
// ここの関数は、前者(gRPC)を後者(GraphQL)へ詰め替えるだけで、業務ロジックや通信は持たない。
//
//	orcan-api --gRPC--> orcanpb.Product --converters--> model.Product --GraphQL--> クライアント
//
// 型の違い(uint32 と Int)、時刻の書式、nil の扱いをここに集めておくと、
// リゾルバーが変換を書き散らさずに済み、gRPC 側のフィールドが増えたときの修正もここだけで済む。
package converters

import "time"

// FormatTimestamp は時刻を、GraphQL で返す文字列(RFC3339)にする。
func FormatTimestamp(timestamp time.Time) string {
	return timestamp.Format(time.RFC3339)
}

// StringOrEmpty は GraphQL の省略可能な入力(*string)を、gRPC で使う文字列にする。省略(nil)は空文字になる。
func StringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
