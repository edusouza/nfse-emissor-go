// Package mongodb provides MongoDB repository implementations for the NFS-e API.
package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// emissionRequestsCollection is the name of the emission requests collection.
	emissionRequestsCollection = "emission_requests"
)

// ErrEmissionRequestNotFound is returned when an emission request is not found.
var ErrEmissionRequestNotFound = errors.New("emission request not found")

// EmissionRequest represents an emission request stored in MongoDB.
type EmissionRequest struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	RequestID   string             `bson:"request_id"`
	APIKeyID    primitive.ObjectID `bson:"api_key_id"`
	Status      string             `bson:"status"`
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
	ProcessedAt *time.Time         `bson:"processed_at,omitempty"`
	Environment string             `bson:"environment"`

	// Provider information
	Provider ProviderData `bson:"provider"`

	// Taker information (optional)
	Taker *TakerData `bson:"taker,omitempty"`

	// Service information
	Service ServiceData `bson:"service"`

	// Monetary values
	Values ValuesData `bson:"values"`

	// DPS information
	DPS DPSData `bson:"dps"`

	// Certificate information (optional, for signed emissions)
	Certificate *CertificateData `bson:"certificate,omitempty"`

	// Webhook configuration
	WebhookURL string `bson:"webhook_url,omitempty"`

	// Processing tracking
	RetryCount int    `bson:"retry_count"`
	LastError  string `bson:"last_error,omitempty"`

	// Result (only on success)
	Result *EmissionResult `bson:"result,omitempty"`

	// Rejection (only on failure)
	Rejection *RejectionInfo `bson:"rejection,omitempty"`

	// Pre-signed XML fields (Phase 5 - User Story 3)
	// IsPreSigned indicates if this request was submitted with pre-signed XML.
	IsPreSigned bool `bson:"is_presigned"`

	// PreSignedXML contains the pre-signed XML content when IsPreSigned is true.
	// This XML is already signed and should be submitted directly to SEFIN
	// without building or signing.
	PreSignedXML string `bson:"presigned_xml,omitempty"`
}

// ProviderData contains provider information for storage.
type ProviderData struct {
	CNPJ                  string `bson:"cnpj"`
	TaxRegime             string `bson:"tax_regime"`
	Name                  string `bson:"name"`
	MunicipalRegistration string `bson:"municipal_registration,omitempty"`
}

// TakerData contains taker information for storage.
type TakerData struct {
	CNPJ string `bson:"cnpj,omitempty"`
	CPF  string `bson:"cpf,omitempty"`
	NIF  string `bson:"nif,omitempty"`
	Name string `bson:"name"`
}

// ServiceData contains service information for storage.
type ServiceData struct {
	NationalCode     string `bson:"national_code"`
	Description      string `bson:"description"`
	MunicipalityCode string `bson:"municipality_code"`
}

// ValuesData contains monetary values for storage.
type ValuesData struct {
	ServiceValue          float64 `bson:"service_value"`
	UnconditionalDiscount float64 `bson:"unconditional_discount,omitempty"`
	ConditionalDiscount   float64 `bson:"conditional_discount,omitempty"`
	Deductions            float64 `bson:"deductions,omitempty"`
}

// DPSData contains DPS information for storage.
type DPSData struct {
	Series string `bson:"series"`
	Number string `bson:"number"`
}

// CertificateData contains certificate information for storage.
// Note: We store only metadata, not the actual certificate data for security.
type CertificateData struct {
	// HasCertificate indicates whether a certificate was provided.
	HasCertificate bool `bson:"has_certificate"`

	// PFXBase64 is the base64-encoded PFX data (encrypted at rest in production).
	// This field is only populated during request processing and should be
	// cleared after signing is complete.
	PFXBase64 string `bson:"pfx_base64,omitempty"`

	// Password is the certificate password (encrypted at rest in production).
	// This field is only populated during request processing and should be
	// cleared after signing is complete.
	Password string `bson:"password,omitempty"`

	// SubjectCN is the Common Name from the certificate subject (for audit).
	SubjectCN string `bson:"subject_cn,omitempty"`

	// IssuerCN is the Common Name from the certificate issuer (for audit).
	IssuerCN string `bson:"issuer_cn,omitempty"`

	// SerialNumber is the certificate serial number (for audit).
	SerialNumber string `bson:"serial_number,omitempty"`

	// IsSigned indicates whether the DPS was signed with this certificate.
	IsSigned bool `bson:"is_signed"`
}

// EmissionResult contains the successful emission result.
type EmissionResult struct {
	NFSeAccessKey string `bson:"nfse_access_key"`
	NFSeNumber    string `bson:"nfse_number"`
	NFSeXML       string `bson:"nfse_xml,omitempty"`
	NFSeXMLURL    string `bson:"nfse_xml_url,omitempty"`
}

// RejectionInfo contains information about a failed emission.
type RejectionInfo struct {
	Code           string `bson:"code"`
	Message        string `bson:"message"`
	GovernmentCode string `bson:"government_code,omitempty"`
	Details        string `bson:"details,omitempty"`
}

// EmissionRepository provides access to emission request data in MongoDB.
type EmissionRepository struct {
	collection *mongo.Collection
}

// NewEmissionRepository creates a new emission repository.
func NewEmissionRepository(client *Client) *EmissionRepository {
	return &EmissionRepository{
		collection: client.GetCollection(emissionRequestsCollection),
	}
}

// Create inserts a new emission request into the database.
func (r *EmissionRepository) Create(ctx context.Context, req *EmissionRequest) error {
	if req == nil {
		return fmt.Errorf("emission request cannot be nil")
	}

	if req.RequestID == "" {
		return fmt.Errorf("request ID is required")
	}

	if req.APIKeyID.IsZero() {
		return fmt.Errorf("API key ID is required")
	}

	// Set timestamps
	now := time.Now().UTC()
	req.CreatedAt = now
	req.UpdatedAt = now

	// Set default status if not provided
	if req.Status == "" {
		req.Status = "pending"
	}

	result, err := r.collection.InsertOne(ctx, req)
	if err != nil {
		// Check for duplicate key error (request_id should be unique)
		if mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("emission request with ID %s already exists", req.RequestID)
		}
		return fmt.Errorf("failed to create emission request: %w", err)
	}

	// Set the generated ID
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		req.ID = oid
	}

	return nil
}

// FindByRequestID retrieves an emission request by its request ID.
func (r *EmissionRepository) FindByRequestID(ctx context.Context, requestID string) (*EmissionRequest, error) {
	if requestID == "" {
		return nil, fmt.Errorf("request ID cannot be empty")
	}

	filter := bson.M{"request_id": requestID}

	var req EmissionRequest
	err := r.collection.FindOne(ctx, filter).Decode(&req)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrEmissionRequestNotFound
		}
		return nil, fmt.Errorf("failed to find emission request: %w", err)
	}

	return &req, nil
}

// UpdateStatus updates the status of an emission request.
func (r *EmissionRepository) UpdateStatus(ctx context.Context, requestID, status string) error {
	if requestID == "" {
		return fmt.Errorf("request ID cannot be empty")
	}

	if status == "" {
		return fmt.Errorf("status cannot be empty")
	}

	filter := bson.M{"request_id": requestID}
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now().UTC(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update emission request status: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrEmissionRequestNotFound
	}

	return nil
}

// UpdateResult updates an emission request with a successful result.
func (r *EmissionRepository) UpdateResult(ctx context.Context, requestID string, result *EmissionResult) error {
	if requestID == "" {
		return fmt.Errorf("request ID cannot be empty")
	}

	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}

	now := time.Now().UTC()
	filter := bson.M{"request_id": requestID}
	update := bson.M{
		"$set": bson.M{
			"status":       "success",
			"result":       result,
			"updated_at":   now,
			"processed_at": now,
		},
	}

	updateResult, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update emission request result: %w", err)
	}

	if updateResult.MatchedCount == 0 {
		return ErrEmissionRequestNotFound
	}

	return nil
}

// UpdateRejection updates an emission request with a rejection/failure result.
func (r *EmissionRepository) UpdateRejection(ctx context.Context, requestID string, rejection *RejectionInfo) error {
	if requestID == "" {
		return fmt.Errorf("request ID cannot be empty")
	}

	if rejection == nil {
		return fmt.Errorf("rejection cannot be nil")
	}

	now := time.Now().UTC()
	filter := bson.M{"request_id": requestID}
	update := bson.M{
		"$set": bson.M{
			"status":       "failed",
			"rejection":    rejection,
			"last_error":   rejection.Message,
			"updated_at":   now,
			"processed_at": now,
		},
		"$inc": bson.M{
			"retry_count": 1,
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update emission request rejection: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrEmissionRequestNotFound
	}

	return nil
}

// UpdateSigningStatus updates the certificate signing status and clears sensitive certificate data.
// This should be called after signing is complete to remove the stored certificate credentials.
func (r *EmissionRepository) UpdateSigningStatus(ctx context.Context, requestID string, isSigned bool, subjectCN, issuerCN, serialNumber string) error {
	if requestID == "" {
		return fmt.Errorf("request ID cannot be empty")
	}

	filter := bson.M{"request_id": requestID}
	update := bson.M{
		"$set": bson.M{
			"certificate.is_signed":     isSigned,
			"certificate.subject_cn":    subjectCN,
			"certificate.issuer_cn":     issuerCN,
			"certificate.serial_number": serialNumber,
			"updated_at":                time.Now().UTC(),
		},
		"$unset": bson.M{
			// Clear sensitive data after signing
			"certificate.pfx_base64": "",
			"certificate.password":   "",
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update signing status: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrEmissionRequestNotFound
	}

	return nil
}

// IncrementRetryCount increments the retry counter and updates the last error.
func (r *EmissionRepository) IncrementRetryCount(ctx context.Context, requestID, lastError string) error {
	if requestID == "" {
		return fmt.Errorf("request ID cannot be empty")
	}

	filter := bson.M{"request_id": requestID}
	update := bson.M{
		"$set": bson.M{
			"last_error": lastError,
			"updated_at": time.Now().UTC(),
		},
		"$inc": bson.M{
			"retry_count": 1,
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to increment retry count: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrEmissionRequestNotFound
	}

	return nil
}

// PaginationParams contains pagination parameters for list queries.
type PaginationParams struct {
	Page     int64
	PageSize int64
}

// PaginatedResult contains paginated query results.
type PaginatedResult struct {
	Items      []*EmissionRequest
	TotalCount int64
	Page       int64
	PageSize   int64
	TotalPages int64
}

// FindByAPIKeyID retrieves emission requests for a specific API key with pagination.
func (r *EmissionRepository) FindByAPIKeyID(ctx context.Context, apiKeyID primitive.ObjectID, params PaginationParams) (*PaginatedResult, error) {
	if apiKeyID.IsZero() {
		return nil, fmt.Errorf("API key ID cannot be empty")
	}

	// Set defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}

	filter := bson.M{"api_key_id": apiKeyID}

	// Get total count
	totalCount, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count emission requests: %w", err)
	}

	// Calculate pagination
	skip := (params.Page - 1) * params.PageSize
	totalPages := (totalCount + params.PageSize - 1) / params.PageSize

	// Find with pagination
	opts := options.Find().
		SetSkip(skip).
		SetLimit(params.PageSize).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find emission requests: %w", err)
	}
	defer cursor.Close(ctx)

	var items []*EmissionRequest
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("failed to decode emission requests: %w", err)
	}

	return &PaginatedResult{
		Items:      items,
		TotalCount: totalCount,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// FindPendingRequests retrieves pending emission requests for processing.
func (r *EmissionRepository) FindPendingRequests(ctx context.Context, limit int64) ([]*EmissionRequest, error) {
	if limit < 1 {
		limit = 10
	}

	filter := bson.M{
		"status": "pending",
	}

	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find pending requests: %w", err)
	}
	defer cursor.Close(ctx)

	var items []*EmissionRequest
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("failed to decode pending requests: %w", err)
	}

	return items, nil
}

// EnsureIndexes creates the necessary indexes for the emission requests collection.
func (r *EmissionRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "request_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "api_key_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
		{
			Keys: bson.D{
				{Key: "api_key_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "created_at", Value: 1},
			},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}
