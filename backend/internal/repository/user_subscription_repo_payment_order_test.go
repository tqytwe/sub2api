package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newUserSubscriptionEntRepo(t *testing.T) (*userSubscriptionRepository, *dbent.Client) {
	t.Helper()

	dbName := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared&_fk=1",
		strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()),
	)
	db, err := sql.Open("sqlite", dbName)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	repo, ok := NewUserSubscriptionRepository(client).(*userSubscriptionRepository)
	require.True(t, ok)
	return repo, client
}

func TestUserSubscriptionRepositoryListByUserIDAttachesPaymentPurchaseOrder(t *testing.T) {
	ctx := context.Background()
	repo, client := newUserSubscriptionEntRepo(t)
	now := time.Date(2026, 7, 18, 15, 39, 56, 0, time.UTC)

	user, err := client.User.Create().
		SetEmail("subscription-buyer@example.com").
		SetPasswordHash("hash").
		SetUsername("subscription-buyer").
		SetStatus(service.StatusActive).
		SetRole(service.RoleUser).
		Save(ctx)
	require.NoError(t, err)

	group, err := client.Group.Create().
		SetName("pro订阅套餐").
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(20).
		SetPayAmount(143.0).
		SetFeeRate(0).
		SetRechargeCode("PAY-102").
		SetOutTradeNo("sub2_20260718abcdef").
		SetPaymentType(payment.TypeWxpay).
		SetPaymentTradeNo("wx-trade-102").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(9).
		SetSubscriptionGroupID(group.ID).
		SetSubscriptionDays(30).
		SetStatus(service.OrderStatusCompleted).
		SetPaidAt(now).
		SetCompletedAt(now.Add(2 * time.Minute)).
		SetExpiresAt(now.Add(30 * time.Minute)).
		SetClientIP("127.0.0.1").
		SetSrcHost("www.jisudeng.com").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(order.ID, 10)).
		SetAction("SUBSCRIPTION_ASSIGNED").
		SetDetail(fmt.Sprintf(`{"groupID":%d,"validityDays":30}`, group.ID)).
		SetOperator("system").
		SetCreatedAt(now.Add(time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(group.ID).
		SetStartsAt(now).
		SetExpiresAt(now.AddDate(0, 0, 30)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(now.Add(time.Minute)).
		SetNotes(fmt.Sprintf("created by checkout\npayment order %d", order.ID)).
		Save(ctx)
	require.NoError(t, err)

	subs, err := repo.ListByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, subs, 1)
	require.NotNil(t, subs[0].PurchaseOrder)
	require.Equal(t, order.ID, subs[0].PurchaseOrder.ID)
	require.Equal(t, "sub2_20260718abcdef", subs[0].PurchaseOrder.OutTradeNo)
	require.Equal(t, payment.TypeWxpay, subs[0].PurchaseOrder.PaymentType)
	require.Equal(t, 143.0, subs[0].PurchaseOrder.PayAmount)
	require.Equal(t, 30, *subs[0].PurchaseOrder.SubscriptionDays)
	require.Equal(t, "SUBSCRIPTION_ASSIGNED", subs[0].PurchaseOrder.AuditAction)
	require.NotNil(t, subs[0].PurchaseOrder.AuditAt)
}
