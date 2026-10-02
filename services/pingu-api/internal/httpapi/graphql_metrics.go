package httpapi

import (
	"context"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/asamigentoku/PinguCoin/pkg/metrics"
)

var (
	graphqlOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_graphql_operations_total",
		Help: "GraphQL の操作の数(種類・ルートのフィールド・結果別)。result は ok / error。",
	}, []string{"type", "field", "result"})

	graphqlDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pingucoin_graphql_operation_duration_seconds",
		Help:    "GraphQL の操作の処理時間(秒。種類別)。",
		Buckets: metrics.LatencyBuckets,
	}, []string{"type"})
)

// invalidOperation は、解釈できなかった(パースや検証に失敗した)リクエストのラベル。
const invalidOperation = "invalid"

// rootFields は、操作の最上位のフィールド名(例: products, createProduct)。
// クライアントが自由に決められる「操作の名前」(query Foo { ... } の Foo)は、ラベルに使わない。
// 任意の文字列で、時系列を、際限なく増やされるのを防ぐため。フィールド名は、スキーマで決まっているので、有限。
// イントロスペクション(__schema など)は、1つにまとめる。
func rootFields(operation *ast.OperationDefinition) []string {
	var names []string
	for _, selection := range operation.SelectionSet {
		field, ok := selection.(*ast.Field)
		if !ok {
			continue
		}
		if strings.HasPrefix(field.Name, "__") {
			names = append(names, "__introspection")
			continue
		}
		names = append(names, field.Name)
	}
	return names
}

// operationMetrics は、GraphQL の操作の数・結果・処理時間を測る gqlgen の拡張(graphqlServer.AroundResponses)。
// HTTP のステータスは、GraphQL では、エラーでも 200 になるので、HTTP のメトリクスだけでは、失敗が見えない。
// この拡張が、レスポンスの errors を見て、結果(ok / error)を数える。
func operationMetrics(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	start := time.Now()
	response := next(ctx)

	if !graphql.HasOperationContext(ctx) {
		graphqlOperations.WithLabelValues(invalidOperation, invalidOperation, "error").Inc()
		return response
	}
	operation := graphql.GetOperationContext(ctx).Operation
	if operation == nil {
		graphqlOperations.WithLabelValues(invalidOperation, invalidOperation, "error").Inc()
		return response
	}

	result := "ok"
	if response != nil && len(response.Errors) > 0 {
		result = "error"
	}
	operationType := string(operation.Operation)
	graphqlDuration.WithLabelValues(operationType).Observe(time.Since(start).Seconds())
	for _, field := range rootFields(operation) {
		graphqlOperations.WithLabelValues(operationType, field, result).Inc()
	}
	return response
}
