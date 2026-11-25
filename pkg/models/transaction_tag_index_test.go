package models

import (
	"sort"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
)

func TestTransactionTagGroupInfoResponseSliceLess(t *testing.T) {
	var transactionTagGroupRespSlice TransactionTagGroupInfoResponseSlice
	transactionTagGroupRespSlice = append(transactionTagGroupRespSlice, &TransactionTagGroupInfoResponse{
		Id:           1,
		DisplayOrder: 3,
	})
	transactionTagGroupRespSlice = append(transactionTagGroupRespSlice, &TransactionTagGroupInfoResponse{
		Id:           2,
		DisplayOrder: 1,
	})
	transactionTagGroupRespSlice = append(transactionTagGroupRespSlice, &TransactionTagGroupInfoResponse{
		Id:           3,
		DisplayOrder: 2,
	})

	sort.Sort(transactionTagGroupRespSlice)

	assert.Equal(t, int64(2), transactionTagGroupRespSlice[0].Id)
	assert.Equal(t, int64(3), transactionTagGroupRespSlice[1].Id)
	assert.Equal(t, int64(1), transactionTagGroupRespSlice[2].Id)
}

func TestTransactionTagIndexSoftDeleteAllowsRepeatedAssociations(t *testing.T) {
	engine, err := xorm.NewEngine("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory sqlite engine: %v", err)
	}
	t.Cleanup(func() {
		_ = engine.Close()
	})

	if err := engine.Sync2(new(TransactionTagIndex)); err != nil {
		t.Fatalf("failed to sync transaction tag index table: %v", err)
	}

	addTag := func(id int64) {
		t.Helper()
		_, err := engine.Insert(&TransactionTagIndex{
			TagIndexId:    id,
			Uid:           1,
			TransactionId: 100,
			TagId:         7,
		})
		if err != nil {
			t.Fatalf("failed to add tag association: %v", err)
		}
	}

	removeTag := func() {
		t.Helper()
		_, err := engine.Cols("deleted").Where("uid=? AND deleted=? AND transaction_id=? AND tag_id=?", 1, false, 100, 7).Update(&TransactionTagIndex{Deleted: true})
		if err != nil {
			t.Fatalf("failed to soft-delete tag association: %v", err)
		}
	}

	addTag(1)
	removeTag()
	addTag(2)
	removeTag()

	var count int64
	count, err = engine.Where("uid=? AND transaction_id=? AND tag_id=?", 1, 100, 7).Count(&TransactionTagIndex{})
	if err != nil {
		t.Fatalf("failed to count tag associations: %v", err)
	}

	assert.Equal(t, int64(2), count)
}
