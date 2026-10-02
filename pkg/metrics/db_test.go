package metrics

import (
	"strings"
	"testing"

	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/testutil"
)

type metricsRow struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func (metricsRow) TableName() string { return "metrics_rows" }

// GORM のプラグインが、操作(create / query / update / delete)・テーブル・結果で、クエリを数える。
// 該当なし(ErrRecordNotFound)は、エラーではなく、not_found として数える(正常な結果なので、エラー率を上げない)。
func TestGORMPlugin_CountsQueries(t *testing.T) {
	db := testutil.NewDB(t, func(db *gorm.DB) error { return db.AutoMigrate(&metricsRow{}) })
	if err := db.Use(GORMPlugin()); err != nil {
		t.Fatal(err)
	}

	count := func(operation, result string) float64 {
		return promtestutil.ToFloat64(dbQueries.WithLabelValues(operation, "metrics_rows", result))
	}
	createBefore := count("create", "ok")
	queryBefore := count("query", "ok")
	notFoundBefore := count("query", "not_found")
	updateBefore := count("update", "ok")
	deleteBefore := count("delete", "ok")
	errorBefore := count("query", "error")

	row := metricsRow{Name: "a"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	var found metricsRow
	if err := db.First(&found, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&found, 999999).Error; err == nil {
		t.Fatal("expected record not found")
	}
	if err := db.Model(&row).Update("name", "b").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&row).Error; err != nil {
		t.Fatal(err)
	}
	// 存在しない列を読む = SQL のエラー。
	if err := db.Table("metrics_rows").Select("no_such_column").Find(&[]metricsRow{}).Error; err == nil {
		t.Fatal("expected a SQL error")
	}

	checks := []struct {
		name         string
		got, before  float64
		wantIncrease float64
	}{
		{"create ok", count("create", "ok"), createBefore, 1},
		{"query ok", count("query", "ok"), queryBefore, 1},
		{"query not_found", count("query", "not_found"), notFoundBefore, 1},
		{"update ok", count("update", "ok"), updateBefore, 1},
		{"delete ok", count("delete", "ok"), deleteBefore, 1},
		{"query error", count("query", "error"), errorBefore, 1},
	}
	for _, check := range checks {
		if got := check.got - check.before; got != check.wantIncrease {
			t.Errorf("%s increased by %v, want %v", check.name, got, check.wantIncrease)
		}
	}
}

// 接続プールの状態は、スクレイプのたびに、その時点の値を読む。
func TestRegisterDBStats_ReportsPool(t *testing.T) {
	db := testutil.NewDB(t, func(db *gorm.DB) error { return nil })
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(7)

	RegisterDBStats(sqlDB)
	RegisterDBStats(sqlDB) // 2回目(置き換え)でも、パニックしない

	collector := newDBStatsCollector(sqlDB)
	if series := promtestutil.CollectAndCount(collector); series != 9 {
		t.Errorf("the collector exposes %d series, want 9", series)
	}
	expected := `# HELP pingucoin_db_connections_max_open 開ける接続の数の上限(0 は無制限)。
# TYPE pingucoin_db_connections_max_open gauge
pingucoin_db_connections_max_open 7
`
	if err := promtestutil.CollectAndCompare(collector, strings.NewReader(expected), "pingucoin_db_connections_max_open"); err != nil {
		t.Error(err)
	}
}
