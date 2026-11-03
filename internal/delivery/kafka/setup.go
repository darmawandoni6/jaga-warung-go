package kafka

import (
	"gorm.io/gorm"

	"jaga-warung-go/internal/repository"
	"jaga-warung-go/internal/usecase"
	pkgkafka "jaga-warung-go/pkg/kafka"
)

// Setup wires the worker: delivery → usecase → repository, the same shape as
// http.SetupRouter. The entrypoint owns config, DB and the broker; this file
// owns which handler a topic dispatches to and the loop's failure policy.
func Setup(reader *pkgkafka.Consumer, db *gorm.DB, pub *pkgkafka.Producer) *Loop {
	stock := usecase.NewProductUsecase(
		repository.NewProductRepo(db),
		// Only needed to satisfy the constructor: the worker never touches
		// categories.
		repository.NewCategoryRepository(db),
	)
	return &Loop{
		fetch:    reader.Fetch,
		commit:   reader.Commit,
		dispatch: NewDispatcher(NewTransactionHandler(stock)),
		publish:  pub.Publish,
		backoff:  retryBackoff,
	}
}
