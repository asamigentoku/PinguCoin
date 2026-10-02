package metrics

import (
	"database/sql"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

// DB の接続プールの状態(sql.DBStats)。スクレイプのたびに、その時点の値を読む。
//
// 見るべきもの:
//   - in_use が max_open に張りついている ... 接続が足りない(リクエストが、接続待ちで詰まっている)
//   - wait_count / wait_duration が増えている ... 実際に、接続を待たされている
//   - open が、突然 0 になる ... DB につながらなくなった
type dbStatsCollector struct {
	db *sql.DB

	open, inUse, idle, maxOpen *prometheus.Desc
	waitCount, waitSeconds     *prometheus.Desc
	closedIdle, closedLifetime *prometheus.Desc
	closedIdleTime             *prometheus.Desc
}

func newDBStatsCollector(db *sql.DB) *dbStatsCollector {
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc("pingucoin_db_"+name, help, nil, nil)
	}
	return &dbStatsCollector{
		db:             db,
		open:           desc("connections_open", "開いている接続の数(使用中 + アイドル)。"),
		inUse:          desc("connections_in_use", "使用中の接続の数。"),
		idle:           desc("connections_idle", "アイドル(待機中)の接続の数。"),
		maxOpen:        desc("connections_max_open", "開ける接続の数の上限(0 は無制限)。"),
		waitCount:      desc("connection_wait_total", "接続が空くのを待たされた回数の合計。"),
		waitSeconds:    desc("connection_wait_seconds_total", "接続が空くのを待った時間の合計(秒)。"),
		closedIdle:     desc("connections_closed_max_idle_total", "アイドルの上限を超えて、閉じた接続の合計。"),
		closedLifetime: desc("connections_closed_max_lifetime_total", "寿命(ConnMaxLifetime)で、閉じた接続の合計。"),
		closedIdleTime: desc("connections_closed_max_idle_time_total", "アイドルの時間(ConnMaxIdleTime)で、閉じた接続の合計。"),
	}
}

func (c *dbStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{c.open, c.inUse, c.idle, c.maxOpen, c.waitCount, c.waitSeconds, c.closedIdle, c.closedLifetime, c.closedIdleTime} {
		ch <- d
	}
}

func (c *dbStatsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.db.Stats()
	gauge := func(d *prometheus.Desc, v float64) { ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v) }
	counter := func(d *prometheus.Desc, v float64) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, v)
	}

	gauge(c.open, float64(stats.OpenConnections))
	gauge(c.inUse, float64(stats.InUse))
	gauge(c.idle, float64(stats.Idle))
	gauge(c.maxOpen, float64(stats.MaxOpenConnections))
	counter(c.waitCount, float64(stats.WaitCount))
	counter(c.waitSeconds, stats.WaitDuration.Seconds())
	counter(c.closedIdle, float64(stats.MaxIdleClosed))
	counter(c.closedLifetime, float64(stats.MaxLifetimeClosed))
	counter(c.closedIdleTime, float64(stats.MaxIdleTimeClosed))
}

// RegisterDBStats は、DB の接続プールのメトリクスを登録する。DB に接続したあとに、1回呼ぶ。
// すでに登録済みのとき(テストなどで、2回目に呼ばれたとき)は、新しい DB のものに、置き換える。
func RegisterDBStats(db *sql.DB) {
	collector := newDBStatsCollector(db)
	if err := registerer.Register(collector); err != nil {
		var already prometheus.AlreadyRegisteredError
		if errors.As(err, &already) {
			registerer.Unregister(already.ExistingCollector)
			_ = registerer.Register(collector)
		}
	}
}

var (
	dbQueries = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_db_queries_total",
		Help: "DB のクエリの数(操作・テーブル・結果別)。result は ok / not_found(該当なし。正常)/ error。",
	}, []string{"operation", "table", "result"})

	dbQueryDuration = factory().NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pingucoin_db_query_duration_seconds",
		Help:    "DB のクエリの処理時間(秒)。",
		Buckets: []float64{.001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"operation", "table"})
)

// GORMPlugin は、GORM の、すべてのクエリの数と処理時間を測るプラグイン(db.Use で付ける)。
// 操作(create / query / update / delete / row / raw)と、テーブルでラベルを付ける。SQL の本文や値は、ラベルに入れない。
func GORMPlugin() gorm.Plugin { return gormPlugin{} }

type gormPlugin struct{}

func (gormPlugin) Name() string { return "pingucoin:metrics" }

const startKey = "pingucoin:metrics:start"

func (gormPlugin) Initialize(db *gorm.DB) error {
	callbacks := db.Callback()

	before := func(tx *gorm.DB) { tx.InstanceSet(startKey, time.Now()) }
	after := func(operation string) func(*gorm.DB) {
		return func(tx *gorm.DB) {
			value, ok := tx.InstanceGet(startKey)
			if !ok {
				return
			}
			start, _ := value.(time.Time)
			table := tx.Statement.Table
			if table == "" {
				table = "unknown"
			}
			result := "ok"
			switch {
			case errors.Is(tx.Error, gorm.ErrRecordNotFound):
				result = "not_found"
			case tx.Error != nil:
				result = "error"
			}
			dbQueryDuration.WithLabelValues(operation, table).Observe(time.Since(start).Seconds())
			dbQueries.WithLabelValues(operation, table, result).Inc()
		}
	}

	steps := []struct {
		operation string
		before    func(string, func(*gorm.DB)) error
		after     func(string, func(*gorm.DB)) error
	}{
		{"create",
			func(n string, f func(*gorm.DB)) error { return callbacks.Create().Before("gorm:create").Register(n, f) },
			func(n string, f func(*gorm.DB)) error {
				return callbacks.Create().After("gorm:after_create").Register(n, f)
			}},
		{"query",
			func(n string, f func(*gorm.DB)) error { return callbacks.Query().Before("gorm:query").Register(n, f) },
			func(n string, f func(*gorm.DB)) error {
				return callbacks.Query().After("gorm:after_query").Register(n, f)
			}},
		{"update",
			func(n string, f func(*gorm.DB)) error { return callbacks.Update().Before("gorm:update").Register(n, f) },
			func(n string, f func(*gorm.DB)) error {
				return callbacks.Update().After("gorm:after_update").Register(n, f)
			}},
		{"delete",
			func(n string, f func(*gorm.DB)) error { return callbacks.Delete().Before("gorm:delete").Register(n, f) },
			func(n string, f func(*gorm.DB)) error {
				return callbacks.Delete().After("gorm:after_delete").Register(n, f)
			}},
		{"row",
			func(n string, f func(*gorm.DB)) error { return callbacks.Row().Before("gorm:row").Register(n, f) },
			func(n string, f func(*gorm.DB)) error { return callbacks.Row().After("gorm:row").Register(n, f) }},
		{"raw",
			func(n string, f func(*gorm.DB)) error { return callbacks.Raw().Before("gorm:raw").Register(n, f) },
			func(n string, f func(*gorm.DB)) error { return callbacks.Raw().After("gorm:raw").Register(n, f) }},
	}
	for _, step := range steps {
		if err := step.before("pingucoin:metrics:before_"+step.operation, before); err != nil {
			return err
		}
		if err := step.after("pingucoin:metrics:after_"+step.operation, after(step.operation)); err != nil {
			return err
		}
	}
	return nil
}
