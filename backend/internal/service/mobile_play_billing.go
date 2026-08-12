package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/golang-jwt/jwt/v5"
)

const (
	SettingMobilePlayBillingProducts = "MOBILE_PLAY_BILLING_PRODUCTS_JSON"

	defaultMobilePlayBillingPackageName = "com.jisudeng.chat"
	defaultGoogleOAuthTokenURL          = "https://oauth2.googleapis.com/token"
	defaultGoogleAndroidPublisherURL    = "https://androidpublisher.googleapis.com"
	googleAndroidPublisherScope         = "https://www.googleapis.com/auth/androidpublisher"
	mobilePlayBillingPaymentType        = "google_play"
)

var (
	ErrMobilePlayBillingNotConfigured       = errors.New("PLAY_BILLING_NOT_CONFIGURED")
	ErrMobilePlayBillingVerificationFailed  = errors.New("PLAY_BILLING_VERIFICATION_FAILED")
	ErrMobilePlayBillingDuplicatePurchase   = errors.New("PLAY_BILLING_DUPLICATE_PURCHASE")
	ErrMobilePlayBillingProductNotMapped    = errors.New("PLAY_BILLING_PRODUCT_NOT_MAPPED")
	ErrMobilePlayBillingPurchaseNotApproved = errors.New("PLAY_BILLING_PURCHASE_NOT_APPROVED")
	ErrMobilePlayBillingInvalidProducts     = errors.New("PLAY_BILLING_INVALID_PRODUCTS")

	mobilePlayBillingProductIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.]{0,149}$`)
)

type MobilePlayBillingPurchaseInput struct {
	ProductID       string  `json:"product_id"`
	ProductType     string  `json:"product_type"`
	PurchaseToken   string  `json:"purchase_token"`
	OrderID         string  `json:"order_id"`
	PackageName     string  `json:"package_name"`
	PurchaseTime    int64   `json:"purchase_time"`
	PurchaseState   int     `json:"purchase_state"`
	Acknowledged    bool    `json:"acknowledged"`
	Quantity        int     `json:"quantity"`
	OriginalJSON    string  `json:"original_json"`
	Signature       string  `json:"signature"`
	PlanID          string  `json:"plan_id"`
	Amount          float64 `json:"amount"`
	OrderType       string  `json:"order_type"`
	Locale          string  `json:"locale"`
	ClientRequestID string  `json:"client_request_id"`
}

type MobilePlayBillingPurchaseResult struct {
	Accepted     bool    `json:"accepted"`
	Verified     bool    `json:"verified"`
	Credited     bool    `json:"credited"`
	Consumed     bool    `json:"consumed,omitempty"`
	Acknowledged bool    `json:"acknowledged,omitempty"`
	Consume      bool    `json:"consume,omitempty"`
	Acknowledge  bool    `json:"acknowledge,omitempty"`
	OrderID      string  `json:"order_id,omitempty"`
	Balance      float64 `json:"balance,omitempty"`
	Amount       float64 `json:"amount,omitempty"`
	Message      string  `json:"message,omitempty"`
}

type MobilePlayBillingProductMapping struct {
	ProductID      string  `json:"product_id"`
	ProductType    string  `json:"product_type,omitempty"`
	OrderType      string  `json:"order_type"`
	Amount         float64 `json:"amount,omitempty"`
	PayAmount      float64 `json:"pay_amount,omitempty"`
	Currency       string  `json:"currency,omitempty"`
	PlanID         int64   `json:"plan_id,omitempty"`
	Title          string  `json:"title,omitempty"`
	Description    string  `json:"description,omitempty"`
	FormattedPrice string  `json:"formatted_price,omitempty"`
	OfferToken     string  `json:"offer_token,omitempty"`
	Consumable     *bool   `json:"consumable,omitempty"`
	Enabled        *bool   `json:"enabled,omitempty"`
}

type MobilePlayBillingPublicProduct struct {
	ProductID      string  `json:"product_id"`
	ProductType    string  `json:"product_type"`
	OrderType      string  `json:"order_type"`
	Amount         float64 `json:"amount,omitempty"`
	PayAmount      float64 `json:"pay_amount,omitempty"`
	Currency       string  `json:"currency,omitempty"`
	PlanID         int64   `json:"plan_id,omitempty"`
	Title          string  `json:"title,omitempty"`
	Description    string  `json:"description,omitempty"`
	FormattedPrice string  `json:"formatted_price,omitempty"`
	OfferToken     string  `json:"offer_token,omitempty"`
}

type MobilePlayBillingAdminConfig struct {
	PackageName              string                            `json:"package_name"`
	ServiceAccountConfigured bool                              `json:"service_account_configured"`
	ConfigSource             string                            `json:"config_source"`
	ProductCount             int                               `json:"product_count"`
	EnabledProductCount      int                               `json:"enabled_product_count"`
	Products                 []MobilePlayBillingProductMapping `json:"products"`
	PublicProducts           []MobilePlayBillingPublicProduct  `json:"public_products"`
}

type UpdateMobilePlayBillingProductsRequest struct {
	Products []MobilePlayBillingProductMapping `json:"products"`
}

type MobilePlayBillingConfig struct {
	PackageName        string
	ServiceAccountFile string
	ServiceAccountJSON string
	APIBaseURL         string
	TokenURL           string
	Products           []MobilePlayBillingProductMapping
}

func LoadMobilePlayBillingConfigFromEnv() MobilePlayBillingConfig {
	return MobilePlayBillingConfig{
		PackageName:        strings.TrimSpace(firstNonEmpty(os.Getenv("PLAY_BILLING_PACKAGE_NAME"), os.Getenv("GOOGLE_PLAY_PACKAGE_NAME"), defaultMobilePlayBillingPackageName)),
		ServiceAccountFile: strings.TrimSpace(firstNonEmpty(os.Getenv("PLAY_BILLING_SERVICE_ACCOUNT_FILE"), os.Getenv("GOOGLE_PLAY_SERVICE_ACCOUNT_FILE"))),
		ServiceAccountJSON: strings.TrimSpace(firstNonEmpty(os.Getenv("PLAY_BILLING_SERVICE_ACCOUNT_JSON"), os.Getenv("GOOGLE_PLAY_SERVICE_ACCOUNT_JSON"))),
		APIBaseURL:         strings.TrimRight(strings.TrimSpace(firstNonEmpty(os.Getenv("PLAY_BILLING_API_BASE_URL"), defaultGoogleAndroidPublisherURL)), "/"),
		TokenURL:           strings.TrimSpace(firstNonEmpty(os.Getenv("PLAY_BILLING_TOKEN_URL"), defaultGoogleOAuthTokenURL)),
		Products:           parseMobilePlayBillingProductMappings(os.Getenv(SettingMobilePlayBillingProducts)),
	}
}

func PublicMobilePlayBillingProducts(ctx context.Context, settingRepo SettingRepository) []MobilePlayBillingPublicProduct {
	cfg := LoadMobilePlayBillingConfigFromEnv()
	if products, err := loadMobilePlayBillingProductsFromSettings(ctx, settingRepo); err == nil && len(products) > 0 {
		cfg.Products = products
	}
	return publicMobilePlayBillingProductsFromMappings(cfg.Products)
}

func (s *PaymentConfigService) PublicMobilePlayBillingProducts(ctx context.Context) []MobilePlayBillingPublicProduct {
	if s == nil {
		return PublicMobilePlayBillingProducts(ctx, nil)
	}
	return PublicMobilePlayBillingProducts(ctx, s.settingRepo)
}

func (s *PaymentConfigService) GetMobilePlayBillingAdminConfig(ctx context.Context) (*MobilePlayBillingAdminConfig, error) {
	cfg := LoadMobilePlayBillingConfigFromEnv()
	configSource := "env"
	if products, err := loadMobilePlayBillingProductsFromSettings(ctx, s.settingRepo); err != nil {
		return nil, err
	} else if products != nil {
		cfg.Products = products
		configSource = "settings"
	}
	products := normalizeMobilePlayBillingProducts(cfg.Products)
	publicProducts := publicMobilePlayBillingProductsFromMappings(products)
	return &MobilePlayBillingAdminConfig{
		PackageName:              strings.TrimSpace(firstNonEmpty(cfg.PackageName, defaultMobilePlayBillingPackageName)),
		ServiceAccountConfigured: cfg.ServiceAccountFile != "" || cfg.ServiceAccountJSON != "",
		ConfigSource:             configSource,
		ProductCount:             len(products),
		EnabledProductCount:      len(publicProducts),
		Products:                 products,
		PublicProducts:           publicProducts,
	}, nil
}

func (s *PaymentConfigService) UpdateMobilePlayBillingProducts(ctx context.Context, req UpdateMobilePlayBillingProductsRequest) (*MobilePlayBillingAdminConfig, error) {
	if s == nil || s.settingRepo == nil {
		return nil, ErrMobilePlayBillingNotConfigured
	}
	products, err := NormalizeAndValidateMobilePlayBillingProducts(req.Products)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(products)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, SettingMobilePlayBillingProducts, string(raw)); err != nil {
		return nil, err
	}
	return s.GetMobilePlayBillingAdminConfig(ctx)
}

func NormalizeAndValidateMobilePlayBillingProducts(products []MobilePlayBillingProductMapping) ([]MobilePlayBillingProductMapping, error) {
	normalized := normalizeMobilePlayBillingProducts(products)
	seen := map[string]struct{}{}
	for i := range normalized {
		product := &normalized[i]
		key := product.ProductType + ":" + strings.ToLower(product.ProductID)
		if _, ok := seen[key]; ok {
			return nil, infraerrors.BadRequest("PLAY_BILLING_DUPLICATE_PRODUCT", "Google Play product mappings must be unique by product_id and product_type")
		}
		seen[key] = struct{}{}
		if !mobilePlayBillingProductIDPattern.MatchString(product.ProductID) || product.ProductID == "android.test" || strings.HasPrefix(product.ProductID, "android.test.") {
			return nil, infraerrors.BadRequest("PLAY_BILLING_INVALID_PRODUCT_ID", "Google Play product_id must start with a lowercase letter or number and contain only lowercase letters, numbers, underscores, or dots")
		}
		if product.ProductType != "inapp" && product.ProductType != "subs" {
			return nil, infraerrors.BadRequest("PLAY_BILLING_INVALID_PRODUCT_TYPE", "Google Play product_type must be inapp or subs")
		}
		if product.ProductType == "subs" && len(product.ProductID) > 40 {
			return nil, infraerrors.BadRequest("PLAY_BILLING_INVALID_PRODUCT_ID", "Google Play subscription product_id must be 40 characters or fewer")
		}
		if product.OrderType != payment.OrderTypeBalance && product.OrderType != payment.OrderTypeSubscription {
			return nil, infraerrors.BadRequest("PLAY_BILLING_INVALID_ORDER_TYPE", "Google Play order_type must be balance or subscription")
		}
		if product.OrderType == payment.OrderTypeBalance && !validPositivePlayBillingAmount(product.Amount) {
			return nil, infraerrors.BadRequest("PLAY_BILLING_INVALID_BALANCE_AMOUNT", "balance products must set amount greater than 0")
		}
		if product.OrderType == payment.OrderTypeSubscription && product.PlanID <= 0 {
			return nil, infraerrors.BadRequest("PLAY_BILLING_INVALID_PLAN_ID", "subscription products must map to a valid plan_id")
		}
		if product.PayAmount < 0 || math.IsNaN(product.PayAmount) || math.IsInf(product.PayAmount, 0) {
			return nil, infraerrors.BadRequest("PLAY_BILLING_INVALID_PAY_AMOUNT", "pay_amount must be non-negative")
		}
		if product.Currency == "" {
			product.Currency = "USD"
		}
		if len(product.Currency) != 3 {
			return nil, infraerrors.BadRequest("PLAY_BILLING_INVALID_CURRENCY", "currency must be a 3-letter ISO currency code")
		}
	}
	return normalized, nil
}

func validPositivePlayBillingAmount(v float64) bool {
	return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

type MobilePlayBillingGooglePurchase struct {
	PackageName             string
	ProductID               string
	ProductType             string
	OrderID                 string
	Purchased               bool
	Pending                 bool
	AcknowledgementState    int
	ConsumptionState        int
	PurchaseTimeMillis      int64
	Quantity                int
	RawPurchaseState        any
	RawAcknowledged         any
	LinkedPurchaseTokenHash string
}

type MobilePlayBillingGoogleVerifier interface {
	VerifyGooglePlayBillingPurchase(ctx context.Context, packageName string, input MobilePlayBillingPurchaseInput) (*MobilePlayBillingGooglePurchase, error)
}

type MobilePlayBillingService struct {
	paymentService *PaymentService
	settingRepo    SettingRepository
	googleVerifier MobilePlayBillingGoogleVerifier
}

func NewMobilePlayBillingService(paymentService *PaymentService, settingRepo SettingRepository) *MobilePlayBillingService {
	return &MobilePlayBillingService{paymentService: paymentService, settingRepo: settingRepo}
}

func NewMobilePlayBillingServiceWithVerifier(paymentService *PaymentService, settingRepo SettingRepository, verifier MobilePlayBillingGoogleVerifier) *MobilePlayBillingService {
	return &MobilePlayBillingService{paymentService: paymentService, settingRepo: settingRepo, googleVerifier: verifier}
}

func (s *MobilePlayBillingService) VerifyMobilePlayBillingPurchase(ctx context.Context, userID int64, input MobilePlayBillingPurchaseInput) (*MobilePlayBillingPurchaseResult, error) {
	if s == nil || s.paymentService == nil {
		return nil, ErrMobilePlayBillingNotConfigured
	}
	normalizeMobilePlayBillingPurchaseInput(&input)
	cfg := s.loadConfig(ctx)
	mapping, ok := findMobilePlayBillingProductMapping(cfg.Products, input.ProductID, input.ProductType)
	if !ok {
		return nil, ErrMobilePlayBillingProductNotMapped
	}
	if input.PackageName != "" && !strings.EqualFold(input.PackageName, cfg.PackageName) {
		return nil, ErrMobilePlayBillingVerificationFailed
	}
	verifier := s.googleVerifier
	if verifier == nil {
		var err error
		verifier, err = NewHTTPMobilePlayBillingGoogleVerifier(cfg)
		if err != nil {
			return nil, err
		}
	}
	verified, err := verifier.VerifyGooglePlayBillingPurchase(ctx, cfg.PackageName, input)
	if err != nil {
		return nil, err
	}
	if verified == nil {
		return nil, ErrMobilePlayBillingVerificationFailed
	}
	if verified.Pending {
		return nil, ErrMobilePlayBillingPurchaseNotApproved
	}
	if !verified.Purchased {
		return nil, ErrMobilePlayBillingVerificationFailed
	}
	if !strings.EqualFold(strings.TrimSpace(verified.PackageName), cfg.PackageName) {
		return nil, ErrMobilePlayBillingVerificationFailed
	}
	if !strings.EqualFold(strings.TrimSpace(verified.ProductID), input.ProductID) {
		return nil, ErrMobilePlayBillingVerificationFailed
	}
	return s.paymentService.FulfillVerifiedMobilePlayBillingPurchase(ctx, userID, input, mapping, verified)
}

func (s *MobilePlayBillingService) loadConfig(ctx context.Context) MobilePlayBillingConfig {
	cfg := LoadMobilePlayBillingConfigFromEnv()
	if products, err := loadMobilePlayBillingProductsFromSettings(ctx, s.settingRepo); err == nil && len(products) > 0 {
		cfg.Products = products
	}
	cfg.Products = normalizeMobilePlayBillingProducts(cfg.Products)
	return cfg
}

func normalizeMobilePlayBillingPurchaseInput(req *MobilePlayBillingPurchaseInput) {
	req.ProductID = strings.TrimSpace(req.ProductID)
	req.ProductType = normalizeMobilePlayBillingProductType(req.ProductType)
	if req.ProductType == "" {
		req.ProductType = "inapp"
	}
	req.PurchaseToken = strings.TrimSpace(req.PurchaseToken)
	req.OrderID = strings.TrimSpace(req.OrderID)
	req.PackageName = strings.TrimSpace(req.PackageName)
	req.OriginalJSON = strings.TrimSpace(req.OriginalJSON)
	req.Signature = strings.TrimSpace(req.Signature)
	req.PlanID = strings.TrimSpace(req.PlanID)
	req.OrderType = normalizeMobilePlayBillingOrderType(req.OrderType)
	req.Locale = strings.TrimSpace(req.Locale)
	req.ClientRequestID = strings.TrimSpace(req.ClientRequestID)
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
}

func normalizeMobilePlayBillingProductType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "subscription", "subscriptions", "sub":
		return "subs"
	case "subs", "inapp":
		return normalized
	case "one_time", "one-time", "managed", "consumable":
		return "inapp"
	default:
		return ""
	}
}

func normalizeMobilePlayBillingOrderType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case payment.OrderTypeSubscription, "plan", "package", "membership":
		return payment.OrderTypeSubscription
	case payment.OrderTypeBalance, "recharge", "topup", "top_up", "wallet", "credit":
		return payment.OrderTypeBalance
	default:
		return normalized
	}
}

func loadMobilePlayBillingProductsFromSettings(ctx context.Context, settingRepo SettingRepository) ([]MobilePlayBillingProductMapping, error) {
	if settingRepo == nil {
		return nil, nil
	}
	raw, err := settingRepo.GetValue(ctx, SettingMobilePlayBillingProducts)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return parseMobilePlayBillingProductMappings(raw), nil
}

func parseMobilePlayBillingProductMappings(raw string) []MobilePlayBillingProductMapping {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var products []MobilePlayBillingProductMapping
	if err := json.Unmarshal([]byte(raw), &products); err == nil {
		return products
	}
	var wrapper struct {
		Products []MobilePlayBillingProductMapping `json:"products"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err == nil {
		return wrapper.Products
	}
	return nil
}

func normalizeMobilePlayBillingProducts(products []MobilePlayBillingProductMapping) []MobilePlayBillingProductMapping {
	out := make([]MobilePlayBillingProductMapping, 0, len(products))
	for _, product := range products {
		product.ProductID = strings.TrimSpace(product.ProductID)
		if product.ProductID == "" {
			continue
		}
		product.ProductType = normalizeMobilePlayBillingProductType(product.ProductType)
		if product.ProductType == "" {
			product.ProductType = "inapp"
		}
		product.OrderType = normalizeMobilePlayBillingOrderType(product.OrderType)
		if product.OrderType == "" {
			if product.PlanID > 0 {
				product.OrderType = payment.OrderTypeSubscription
			} else {
				product.OrderType = payment.OrderTypeBalance
			}
		}
		product.Currency = strings.ToUpper(strings.TrimSpace(product.Currency))
		if product.Currency == "" {
			product.Currency = "USD"
		}
		product.Title = strings.TrimSpace(product.Title)
		product.Description = strings.TrimSpace(product.Description)
		product.FormattedPrice = strings.TrimSpace(product.FormattedPrice)
		product.OfferToken = strings.TrimSpace(product.OfferToken)
		out = append(out, product)
	}
	return out
}

func publicMobilePlayBillingProductsFromMappings(products []MobilePlayBillingProductMapping) []MobilePlayBillingPublicProduct {
	public := make([]MobilePlayBillingPublicProduct, 0, len(products))
	for _, product := range normalizeMobilePlayBillingProducts(products) {
		if product.Enabled != nil && !*product.Enabled {
			continue
		}
		public = append(public, MobilePlayBillingPublicProduct{
			ProductID: product.ProductID, ProductType: product.ProductType, OrderType: product.OrderType,
			Amount: product.Amount, PayAmount: product.PayAmount, Currency: product.Currency,
			PlanID: product.PlanID, Title: product.Title, Description: product.Description,
			FormattedPrice: product.FormattedPrice, OfferToken: product.OfferToken,
		})
	}
	return public
}

func findMobilePlayBillingProductMapping(products []MobilePlayBillingProductMapping, productID, productType string) (MobilePlayBillingProductMapping, bool) {
	productID = strings.TrimSpace(productID)
	productType = normalizeMobilePlayBillingProductType(productType)
	for _, product := range normalizeMobilePlayBillingProducts(products) {
		if !strings.EqualFold(product.ProductID, productID) {
			continue
		}
		if product.Enabled != nil && !*product.Enabled {
			continue
		}
		if productType == "" || product.ProductType == productType {
			return product, true
		}
	}
	return MobilePlayBillingProductMapping{}, false
}

func (s *PaymentService) FulfillVerifiedMobilePlayBillingPurchase(
	ctx context.Context,
	userID int64,
	input MobilePlayBillingPurchaseInput,
	mapping MobilePlayBillingProductMapping,
	verified *MobilePlayBillingGooglePurchase,
) (*MobilePlayBillingPurchaseResult, error) {
	if s == nil || s.entClient == nil || s.userRepo == nil || s.redeemService == nil {
		return nil, ErrMobilePlayBillingNotConfigured
	}
	normalizeMobilePlayBillingPurchaseInput(&input)
	normalized := normalizeMobilePlayBillingProducts([]MobilePlayBillingProductMapping{mapping})
	if len(normalized) == 0 {
		return nil, ErrMobilePlayBillingProductNotMapped
	}
	mapping = normalized[0]
	if mapping.OrderType != payment.OrderTypeBalance && mapping.OrderType != payment.OrderTypeSubscription {
		return nil, ErrMobilePlayBillingProductNotMapped
	}
	if mapping.OrderType == payment.OrderTypeBalance && mapping.Amount <= 0 {
		return nil, ErrMobilePlayBillingProductNotMapped
	}
	if mapping.OrderType == payment.OrderTypeSubscription && mapping.PlanID <= 0 {
		return nil, ErrMobilePlayBillingProductNotMapped
	}

	outTradeNo := mobilePlayBillingOutTradeNo(input.PurchaseToken)
	existing, err := s.entClient.PaymentOrder.Query().Where(paymentorder.OutTradeNo(outTradeNo)).Only(ctx)
	if err == nil && existing != nil {
		if existing.UserID != userID {
			return nil, ErrMobilePlayBillingDuplicatePurchase
		}
		if existing.Status == OrderStatusPaid || existing.Status == OrderStatusRecharging {
			if fulfillErr := s.executeFulfillment(ctx, existing.ID); fulfillErr != nil {
				return nil, fulfillErr
			}
		}
		current, getErr := s.entClient.PaymentOrder.Get(ctx, existing.ID)
		if getErr != nil {
			current = existing
		}
		return s.mobilePlayBillingResult(ctx, current, mapping, verified), nil
	}
	if err != nil && !dbent.IsNotFound(err) {
		return nil, err
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user.Status != payment.EntityStatusActive {
		return nil, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
	}

	var plan *dbent.SubscriptionPlan
	if mapping.OrderType == payment.OrderTypeSubscription {
		plan, err = s.configService.GetPlan(ctx, mapping.PlanID)
		if err != nil || plan == nil || !plan.ForSale {
			return nil, ErrMobilePlayBillingProductNotMapped
		}
		group, groupErr := s.groupRepo.GetByID(ctx, plan.GroupID)
		if groupErr != nil || group == nil || group.Status != payment.EntityStatusActive || !group.IsSubscriptionType() {
			return nil, ErrMobilePlayBillingProductNotMapped
		}
	}

	order, err := s.createMobilePlayBillingOrder(ctx, user, input, mapping, verified, outTradeNo, plan)
	if err != nil {
		if dbent.IsConstraintError(err) {
			return nil, ErrMobilePlayBillingDuplicatePurchase
		}
		return nil, err
	}
	if err := s.toPaid(ctx, order, mobilePlayBillingTradeNo(input, verified), mobilePlayBillingPayAmount(mapping), mobilePlayBillingPaymentType, mobilePlayBillingNotificationMetadata(input, mapping, verified)); err != nil {
		return nil, err
	}
	completed, err := s.entClient.PaymentOrder.Get(ctx, order.ID)
	if err != nil {
		completed = order
	}
	return s.mobilePlayBillingResult(ctx, completed, mapping, verified), nil
}

func (s *PaymentService) createMobilePlayBillingOrder(
	ctx context.Context,
	user *User,
	input MobilePlayBillingPurchaseInput,
	mapping MobilePlayBillingProductMapping,
	verified *MobilePlayBillingGooglePurchase,
	outTradeNo string,
	plan *dbent.SubscriptionPlan,
) (*dbent.PaymentOrder, error) {
	now := time.Now().UTC()
	amount := mapping.Amount
	if mapping.OrderType == payment.OrderTypeSubscription && plan != nil {
		amount = plan.Price
	}
	payAmount := mobilePlayBillingPayAmount(mapping)
	if payAmount <= 0 {
		payAmount = amount
	}
	quantity := input.Quantity
	if verified != nil && verified.Quantity > quantity {
		quantity = verified.Quantity
	}
	if quantity <= 0 {
		quantity = 1
	}
	if quantity > 1 && mapping.OrderType == payment.OrderTypeBalance {
		amount *= float64(quantity)
		payAmount *= float64(quantity)
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin play billing order transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	b := tx.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetNillableUserNotes(psNilIfEmpty(user.Notes)).
		SetAmount(amount).
		SetListAmount(payAmount).
		SetGatewayBaseAmount(payAmount).
		SetDiscountAmount(0).
		SetFeeAmount(0).
		SetQualifyingRechargeAmount(payAmount).
		SetPaymentCurrency(strings.ToUpper(strings.TrimSpace(mapping.Currency))).
		SetPayAmount(payAmount).
		SetFeeRate(0).
		SetRechargeCode("").
		SetOutTradeNo(outTradeNo).
		SetPaymentType(mobilePlayBillingPaymentType).
		SetProviderKey(mobilePlayBillingPaymentType).
		SetPaymentTradeNo(mobilePlayBillingTradeNo(input, verified)).
		SetOrderType(mapping.OrderType).
		SetProviderSnapshot(mobilePlayBillingProviderSnapshot(input, mapping, verified)).
		SetStatus(OrderStatusPending).
		SetExpiresAt(now.Add(24 * time.Hour)).
		SetClientIP("mobile-play-billing").
		SetSrcHost("android-play")
	if mapping.OrderType == payment.OrderTypeSubscription && plan != nil {
		validityDays := psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit)
		b.SetPlanID(plan.ID).SetSubscriptionGroupID(plan.GroupID).SetSubscriptionDays(validityDays)
		b.SetSubscriptionSnapshot(map[string]any{
			"plan_id":                  plan.ID,
			"group_id":                 plan.GroupID,
			"validity_days":            validityDays,
			"google_play_product_id":   mapping.ProductID,
			"google_play_product_type": mapping.ProductType,
			"payment_currency":         mapping.Currency,
		})
	}
	order, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create play billing order: %w", err)
	}
	code := fmt.Sprintf("GPAY-%d-%s", order.ID, mobilePlayBillingTokenHash(input.PurchaseToken)[:10])
	order, err = tx.PaymentOrder.UpdateOneID(order.ID).SetRechargeCode(code).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("set play billing recharge code: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit play billing order transaction: %w", err)
	}
	s.writeAuditLog(ctx, order.ID, "PLAY_BILLING_VERIFIED", mobilePlayBillingPaymentType, mobilePlayBillingAuditMetadata(input, mapping, verified))
	return order, nil
}

func (s *PaymentService) mobilePlayBillingResult(ctx context.Context, order *dbent.PaymentOrder, mapping MobilePlayBillingProductMapping, verified *MobilePlayBillingGooglePurchase) *MobilePlayBillingPurchaseResult {
	balance := 0.0
	if s != nil && s.userRepo != nil && order != nil {
		if user, err := s.userRepo.GetByID(ctx, order.UserID); err == nil && user != nil {
			balance = user.Balance
		}
	}
	status := ""
	orderID := ""
	if order != nil {
		status = order.Status
		orderID = strconv.FormatInt(order.ID, 10)
	}
	credited := status == OrderStatusCompleted
	consume := mobilePlayBillingShouldConsume(mapping)
	ack := mobilePlayBillingShouldAcknowledge(mapping, verified)
	return &MobilePlayBillingPurchaseResult{
		Accepted: true, Verified: true, Credited: credited,
		Consumed:     verified != nil && verified.ConsumptionState == 1,
		Acknowledged: verified != nil && verified.AcknowledgementState == 1,
		Consume:      consume, Acknowledge: ack,
		OrderID: orderID, Balance: balance, Amount: mapping.Amount,
		Message: "Play Billing purchase verified and submitted for fulfillment",
	}
}

func mobilePlayBillingPayAmount(mapping MobilePlayBillingProductMapping) float64 {
	if mapping.PayAmount > 0 {
		return mapping.PayAmount
	}
	return mapping.Amount
}

func mobilePlayBillingShouldConsume(mapping MobilePlayBillingProductMapping) bool {
	if mapping.Consumable != nil {
		return *mapping.Consumable
	}
	return mapping.ProductType == "inapp"
}

func mobilePlayBillingShouldAcknowledge(mapping MobilePlayBillingProductMapping, verified *MobilePlayBillingGooglePurchase) bool {
	if verified == nil || verified.AcknowledgementState != 0 {
		return false
	}
	if mapping.ProductType == "subs" {
		return true
	}
	return mapping.ProductType == "inapp" && !mobilePlayBillingShouldConsume(mapping)
}

func mobilePlayBillingOutTradeNo(token string) string {
	return "gplay_" + mobilePlayBillingTokenHash(token)[:48]
}

func mobilePlayBillingTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func mobilePlayBillingTradeNo(input MobilePlayBillingPurchaseInput, verified *MobilePlayBillingGooglePurchase) string {
	if verified != nil && strings.TrimSpace(verified.OrderID) != "" {
		return strings.TrimSpace(verified.OrderID)
	}
	if strings.TrimSpace(input.OrderID) != "" {
		return strings.TrimSpace(input.OrderID)
	}
	return mobilePlayBillingOutTradeNo(input.PurchaseToken)
}

func mobilePlayBillingProviderSnapshot(input MobilePlayBillingPurchaseInput, mapping MobilePlayBillingProductMapping, verified *MobilePlayBillingGooglePurchase) map[string]any {
	snapshot := mobilePlayBillingAuditMetadata(input, mapping, verified)
	snapshot["provider"] = mobilePlayBillingPaymentType
	snapshot["purchase_token_hash"] = mobilePlayBillingTokenHash(input.PurchaseToken)
	return snapshot
}

func mobilePlayBillingAuditMetadata(input MobilePlayBillingPurchaseInput, mapping MobilePlayBillingProductMapping, verified *MobilePlayBillingGooglePurchase) map[string]any {
	metadata := map[string]any{
		"google_play_product_id":   input.ProductID,
		"google_play_product_type": input.ProductType,
		"order_type":               mapping.OrderType,
		"plan_id":                  mapping.PlanID,
		"amount":                   mapping.Amount,
		"pay_amount":               mobilePlayBillingPayAmount(mapping),
		"currency":                 mapping.Currency,
		"purchase_token_hash":      mobilePlayBillingTokenHash(input.PurchaseToken),
		"client_request_id":        input.ClientRequestID,
	}
	if input.OrderID != "" {
		metadata["client_order_id"] = input.OrderID
	}
	if verified != nil {
		metadata["google_order_id"] = verified.OrderID
		metadata["google_purchase_time_millis"] = verified.PurchaseTimeMillis
		metadata["google_acknowledgement_state"] = verified.AcknowledgementState
		metadata["google_consumption_state"] = verified.ConsumptionState
		metadata["google_purchase_state"] = verified.RawPurchaseState
		metadata["google_acknowledged"] = verified.RawAcknowledged
		metadata["google_linked_purchase_token_hash"] = verified.LinkedPurchaseTokenHash
	}
	return metadata
}

func mobilePlayBillingNotificationMetadata(input MobilePlayBillingPurchaseInput, mapping MobilePlayBillingProductMapping, verified *MobilePlayBillingGooglePurchase) map[string]string {
	metadata := map[string]string{
		"google_play_product_id":   input.ProductID,
		"google_play_product_type": input.ProductType,
		"order_type":               mapping.OrderType,
		"purchase_token_hash":      mobilePlayBillingTokenHash(input.PurchaseToken),
		"client_request_id":        input.ClientRequestID,
	}
	if mapping.PlanID > 0 {
		metadata["plan_id"] = strconv.FormatInt(mapping.PlanID, 10)
	}
	if verified != nil && strings.TrimSpace(verified.OrderID) != "" {
		metadata["google_order_id"] = strings.TrimSpace(verified.OrderID)
	}
	return metadata
}

type HTTPMobilePlayBillingGoogleVerifier struct {
	httpClient *http.Client
	cfg        MobilePlayBillingConfig
	mu         sync.Mutex
	token      string
	expiresAt  time.Time
}

func NewHTTPMobilePlayBillingGoogleVerifier(cfg MobilePlayBillingConfig) (*HTTPMobilePlayBillingGoogleVerifier, error) {
	cfg.PackageName = strings.TrimSpace(firstNonEmpty(cfg.PackageName, defaultMobilePlayBillingPackageName))
	cfg.APIBaseURL = strings.TrimRight(strings.TrimSpace(firstNonEmpty(cfg.APIBaseURL, defaultGoogleAndroidPublisherURL)), "/")
	cfg.TokenURL = strings.TrimSpace(firstNonEmpty(cfg.TokenURL, defaultGoogleOAuthTokenURL))
	if cfg.ServiceAccountFile == "" && cfg.ServiceAccountJSON == "" {
		return nil, ErrMobilePlayBillingNotConfigured
	}
	if cfg.ServiceAccountFile != "" && cfg.ServiceAccountJSON != "" {
		return nil, ErrMobilePlayBillingNotConfigured
	}
	return &HTTPMobilePlayBillingGoogleVerifier{cfg: cfg, httpClient: &http.Client{Timeout: 12 * time.Second}}, nil
}

func (v *HTTPMobilePlayBillingGoogleVerifier) VerifyGooglePlayBillingPurchase(ctx context.Context, packageName string, input MobilePlayBillingPurchaseInput) (*MobilePlayBillingGooglePurchase, error) {
	if v == nil {
		return nil, ErrMobilePlayBillingNotConfigured
	}
	token, err := v.accessToken(ctx)
	if err != nil {
		return nil, ErrMobilePlayBillingVerificationFailed
	}
	productType := normalizeMobilePlayBillingProductType(input.ProductType)
	if productType == "subs" {
		return v.verifySubscription(ctx, token, packageName, input)
	}
	return v.verifyProduct(ctx, token, packageName, input)
}

func (v *HTTPMobilePlayBillingGoogleVerifier) verifyProduct(ctx context.Context, accessToken, packageName string, input MobilePlayBillingPurchaseInput) (*MobilePlayBillingGooglePurchase, error) {
	endpoint := fmt.Sprintf("%s/androidpublisher/v3/applications/%s/purchases/products/%s/tokens/%s",
		v.cfg.APIBaseURL,
		url.PathEscape(packageName),
		url.PathEscape(input.ProductID),
		url.PathEscape(input.PurchaseToken),
	)
	var payload struct {
		Kind                 string `json:"kind"`
		PurchaseTimeMillis   string `json:"purchaseTimeMillis"`
		PurchaseState        int    `json:"purchaseState"`
		ConsumptionState     int    `json:"consumptionState"`
		AcknowledgementState int    `json:"acknowledgementState"`
		OrderID              string `json:"orderId"`
		RegionCode           string `json:"regionCode"`
		Quantity             int    `json:"quantity"`
	}
	if err := v.getJSON(ctx, endpoint, accessToken, &payload); err != nil {
		return nil, err
	}
	purchaseTime, _ := strconv.ParseInt(payload.PurchaseTimeMillis, 10, 64)
	return &MobilePlayBillingGooglePurchase{
		PackageName: packageName, ProductID: input.ProductID, ProductType: "inapp",
		OrderID: payload.OrderID, Purchased: payload.PurchaseState == 0, Pending: payload.PurchaseState == 2,
		AcknowledgementState: payload.AcknowledgementState, ConsumptionState: payload.ConsumptionState,
		PurchaseTimeMillis: purchaseTime, Quantity: payload.Quantity, RawPurchaseState: payload.PurchaseState,
	}, nil
}

func (v *HTTPMobilePlayBillingGoogleVerifier) verifySubscription(ctx context.Context, accessToken, packageName string, input MobilePlayBillingPurchaseInput) (*MobilePlayBillingGooglePurchase, error) {
	endpoint := fmt.Sprintf("%s/androidpublisher/v3/applications/%s/purchases/subscriptionsv2/tokens/%s",
		v.cfg.APIBaseURL,
		url.PathEscape(packageName),
		url.PathEscape(input.PurchaseToken),
	)
	var payload struct {
		LatestOrderID        string `json:"latestOrderId"`
		SubscriptionState    string `json:"subscriptionState"`
		AcknowledgementState string `json:"acknowledgementState"`
		StartTime            string `json:"startTime"`
		LineItems            []struct {
			ProductID string `json:"productId"`
		} `json:"lineItems"`
		LinkedPurchaseToken string `json:"linkedPurchaseToken"`
	}
	if err := v.getJSON(ctx, endpoint, accessToken, &payload); err != nil {
		return nil, err
	}
	productID := input.ProductID
	if len(payload.LineItems) > 0 && strings.TrimSpace(payload.LineItems[0].ProductID) != "" {
		productID = strings.TrimSpace(payload.LineItems[0].ProductID)
	}
	ackState := 0
	if strings.EqualFold(payload.AcknowledgementState, "ACKNOWLEDGEMENT_STATE_ACKNOWLEDGED") {
		ackState = 1
	}
	startTime := int64(0)
	if parsed, err := time.Parse(time.RFC3339Nano, payload.StartTime); err == nil {
		startTime = parsed.UnixMilli()
	}
	state := strings.ToUpper(strings.TrimSpace(payload.SubscriptionState))
	return &MobilePlayBillingGooglePurchase{
		PackageName: packageName, ProductID: productID, ProductType: "subs",
		OrderID: payload.LatestOrderID, Purchased: state == "SUBSCRIPTION_STATE_ACTIVE", Pending: state == "SUBSCRIPTION_STATE_PENDING",
		AcknowledgementState: ackState, PurchaseTimeMillis: startTime, Quantity: 1,
		RawPurchaseState: payload.SubscriptionState, RawAcknowledged: payload.AcknowledgementState,
		LinkedPurchaseTokenHash: mobilePlayBillingTokenHash(payload.LinkedPurchaseToken),
	}, nil
}

func (v *HTTPMobilePlayBillingGoogleVerifier) accessToken(ctx context.Context) (string, error) {
	v.mu.Lock()
	if v.token != "" && time.Now().Before(v.expiresAt.Add(-time.Minute)) {
		token := v.token
		v.mu.Unlock()
		return token, nil
	}
	v.mu.Unlock()

	credentials, err := v.serviceAccount()
	if err != nil {
		return "", err
	}
	signed, err := signGoogleServiceAccountJWT(credentials, v.cfg.TokenURL)
	if err != nil {
		return "", err
	}
	values := url.Values{}
	values.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	values.Set("assertion", signed)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.cfg.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("google play oauth token status %d: %s", resp.StatusCode, string(body))
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", errors.New("google play oauth token response missing access_token")
	}
	expiresIn := payload.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	v.mu.Lock()
	v.token = strings.TrimSpace(payload.AccessToken)
	v.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	v.mu.Unlock()
	return payload.AccessToken, nil
}

func (v *HTTPMobilePlayBillingGoogleVerifier) getJSON(ctx context.Context, endpoint, accessToken string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("google play purchase status %d: %s", resp.StatusCode, string(body))
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	return decoder.Decode(dest)
}

type googleServiceAccountCredentials struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

func (v *HTTPMobilePlayBillingGoogleVerifier) serviceAccount() (*googleServiceAccountCredentials, error) {
	raw := strings.TrimSpace(v.cfg.ServiceAccountJSON)
	if raw == "" && v.cfg.ServiceAccountFile != "" {
		data, err := os.ReadFile(v.cfg.ServiceAccountFile)
		if err != nil {
			return nil, err
		}
		raw = string(data)
	}
	var credentials googleServiceAccountCredentials
	if err := json.Unmarshal([]byte(raw), &credentials); err != nil {
		return nil, err
	}
	credentials.ClientEmail = strings.TrimSpace(credentials.ClientEmail)
	credentials.PrivateKey = strings.TrimSpace(credentials.PrivateKey)
	credentials.TokenURI = strings.TrimSpace(firstNonEmpty(credentials.TokenURI, v.cfg.TokenURL, defaultGoogleOAuthTokenURL))
	if credentials.ClientEmail == "" || credentials.PrivateKey == "" {
		return nil, errors.New("google play service account json missing client_email or private_key")
	}
	return &credentials, nil
}

func signGoogleServiceAccountJWT(credentials *googleServiceAccountCredentials, tokenURL string) (string, error) {
	block, _ := pem.Decode([]byte(credentials.PrivateKey))
	if block == nil {
		return "", errors.New("google play service account private key is not PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		rsaKey, rsaErr := x509.ParsePKCS1PrivateKey(block.Bytes)
		if rsaErr != nil {
			return "", err
		}
		key = rsaKey
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   credentials.ClientEmail,
		"scope": googleAndroidPublisherScope,
		"aud":   firstNonEmpty(credentials.TokenURI, tokenURL, defaultGoogleOAuthTokenURL),
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
}
