package fanout

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"runtime/debug"
)

type Entity struct {
	ID         uint32
	ExternalID uuid.UUID
}

type Repository interface {
	ForEachPage(ctx context.Context, offset uint32, cb func(ctx context.Context, entities []*Entity, hasMore bool) error) error
	WithTX(ctx context.Context, cb func(ctx context.Context) error) error
}

type repo struct {
	db *gorm.DB
}

const (
	_limit     = 100
	_batchSize = 25
)

func (r *repo) ForEachPage(
	ctx context.Context,
	offset uint32,
	cb func(ctx context.Context, entities []*Entity, hasMore bool) error,
) error {
	tx := r.TxFromContext(ctx)

	var entities []*Entity
	err := tx.Model(&Entity{}).
		Where("id > ?", offset).
		Limit(_limit).
		FindInBatches(&entities, _batchSize, func(tx *gorm.DB, batch int) error {
			return cb(tx.Statement.Context, entities, len(entities) == _batchSize)
		}).
		Error

	return err
}

type txKey struct{}

func (r *repo) WithTX(ctx context.Context, cb func(ctx2 context.Context) error) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}

	_, ok := ctx.Value(txKey{}).(*gorm.DB)
	if ok {
		panic("a tx already exists in context, nested transactions are not supported")
	}
	txOpts := &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: false}

	// Setting tx.WithContext(ctx) preserves the current context within subsequent operations in this DB session,
	// facilitating a correct trace span hierarchy within the scope of WithTX.
	tx := r.db.Begin(txOpts).WithContext(ctx)
	err = tx.Error

	defer func() {
		if r := recover(); r != nil {
			rollback := tx.Rollback().Error
			if rollback != nil {
				err = rollback
			}
			err = fmt.Errorf("panic recovered: %v\n%s", err, debug.Stack())
			return
		}
	}()

	if err != nil {
		return err
	}

	newCtx := context.WithValue(ctx, txKey{}, tx)

	cbErr := cb(newCtx)

	if cbErr != nil {
		err = tx.Rollback().Error
		if err != nil {
			return err
		}
		return cbErr
	}
	err = tx.Commit().Error

	if err != nil {
		return err
	}

	return err
}

// ErrCtxMissingTx is raised when TxFromContext is called but not tx is found
// in the given context
var ErrCtxMissingTx = errors.New(
	"a tx was expected in your ctx",
)

// GetTXFromContext exposes the gorm.DB from the context in an active transaction. Typically, this is
// used as a utility function for other injectable dependencies to use and build off a transaction.
func GetTXFromContext(ctx context.Context) (*gorm.DB, error) {
	db, ok := ctx.Value(txKey{}).(*gorm.DB)
	if !ok {
		return nil, ErrCtxMissingTx
	}
	return db, nil
}

func (r *repo) TxFromContext(ctx context.Context) *gorm.DB {
	tx, err := GetTXFromContext(ctx)
	if err != nil {
		panic(err)
	}

	return tx
}

func NewRepository(db *gorm.DB) Repository {
	return &repo{db: db}
}
