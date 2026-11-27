package models

import (
	"sort"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
)

func TestTokenInfoResponseSliceLess(t *testing.T) {
	var tokenInfoRespSlice TokenInfoResponseSlice
	tokenInfoRespSlice = append(tokenInfoRespSlice, &TokenInfoResponse{
		TokenId:  "1",
		LastSeen: int64(1),
	})
	tokenInfoRespSlice = append(tokenInfoRespSlice, &TokenInfoResponse{
		TokenId:  "2",
		LastSeen: int64(3),
	})
	tokenInfoRespSlice = append(tokenInfoRespSlice, &TokenInfoResponse{
		TokenId:  "3",
		LastSeen: int64(2),
	})

	sort.Sort(tokenInfoRespSlice)

	assert.Equal(t, "2", tokenInfoRespSlice[0].TokenId)
	assert.Equal(t, "3", tokenInfoRespSlice[1].TokenId)
	assert.Equal(t, "1", tokenInfoRespSlice[2].TokenId)
}

func TestTokenRecordSyncDoesNotCreatePhantomColumn(t *testing.T) {
	engine, err := xorm.NewEngine("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory sqlite engine: %v", err)
	}
	t.Cleanup(func() {
		_ = engine.Close()
	})

	if err := engine.Sync2(new(TokenRecord)); err != nil {
		t.Fatalf("failed to sync token record table: %v", err)
	}

	metas, err := engine.DBMetas()
	if err != nil {
		t.Fatalf("failed to read database metadata: %v", err)
	}

	for _, meta := range metas {
		if meta.Name != "token_record" {
			continue
		}

		for _, column := range meta.ColumnsSeq() {
			if column == "token_record" {
				t.Fatal("token_record phantom column must not be generated")
			}
		}

		return
	}

	t.Fatal("token_record table was not created")
}
