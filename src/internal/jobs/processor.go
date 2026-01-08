// Package jobs provides background job definitions and handlers for the NFS-e API.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"

	"github.com/eduardo/nfse-nacional/internal/domain/emission"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/mongodb"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/sefin"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/webhook"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/xmlsigner"
	"github.com/eduardo/nfse-nacional/pkg/xmlbuilder"
)

// EmissionProcessor handles emission job processing.
type EmissionProcessor struct {
	emissionRepo  *mongodb.EmissionRepository
	webhookRepo   *mongodb.WebhookRepository
	sefinClient   sefin.SefinClient
	webhookSender *webhook.Sender
}

// EmissionProcessorConfig configures the emission processor.
type EmissionProcessorConfig struct {
	// EmissionRepo is the repository for emission requests.
	EmissionRepo *mongodb.EmissionRepository

	// WebhookRepo is the repository for webhook deliveries.
	WebhookRepo *mongodb.WebhookRepository

	// SefinClient is the SEFIN API client.
	SefinClient sefin.SefinClient

	// WebhookSender is the webhook sender.
	WebhookSender *webhook.Sender
}

// NewEmissionProcessor creates a new emission processor.
func NewEmissionProcessor(config EmissionProcessorConfig) *EmissionProcessor {
	return &EmissionProcessor{
		emissionRepo:  config.EmissionRepo,
		webhookRepo:   config.WebhookRepo,
		sefinClient:   config.SefinClient,
		webhookSender: config.WebhookSender,
	}
}

// ProcessEmission handles the emission:process task.
func (p *EmissionProcessor) ProcessEmission(ctx context.Context, task *asynq.Task) error {
	// Parse task payload
	payload, err := ParseEmissionTask(task)
	if err != nil {
		// Return nil to prevent retries for invalid payloads
		log.Printf("Error parsing emission task: %v", err)
		return nil
	}

	requestID := payload.RequestID
	log.Printf("Processing emission request: %s", requestID)

	// Load emission request from database
	emissionReq, err := p.emissionRepo.FindByRequestID(ctx, requestID)
	if err != nil {
		if err == mongodb.ErrEmissionRequestNotFound {
			log.Printf("Emission request not found: %s", requestID)
			return nil // Don't retry if not found
		}
		return fmt.Errorf("failed to load emission request: %w", err)
	}

	// Skip if already processed
	if emissionReq.Status == emission.StatusSuccess || emissionReq.Status == emission.StatusFailed {
		log.Printf("Emission request %s already processed with status: %s", requestID, emissionReq.Status)
		return nil
	}

	// Update status to processing
	if err := p.emissionRepo.UpdateStatus(ctx, requestID, emission.StatusProcessing); err != nil {
		return fmt.Errorf("failed to update status to processing: %w", err)
	}

	// Determine the DPS XML to submit
	var dpsXML string

	// Check if this is a pre-signed XML request (Phase 5 - User Story 3)
	if emissionReq.IsPreSigned && emissionReq.PreSignedXML != "" {
		// Pre-signed flow: Use the stored pre-signed XML directly
		log.Printf("Processing pre-signed XML for request %s", requestID)
		dpsXML = emissionReq.PreSignedXML
	} else {
		// Standard flow: Build and optionally sign the DPS XML
		dpsResult, err := p.buildDPSXML(emissionReq)
		if err != nil {
			// This is a configuration/validation error, don't retry
			rejectionInfo := &mongodb.RejectionInfo{
				Code:    emission.ErrorCodeXMLBuildError,
				Message: fmt.Sprintf("Failed to build DPS XML: %v", err),
			}
			if updateErr := p.emissionRepo.UpdateRejection(ctx, requestID, rejectionInfo); updateErr != nil {
				log.Printf("Error updating rejection: %v", updateErr)
			}
			p.sendWebhook(ctx, emissionReq, nil, rejectionInfo)
			return nil // Don't retry XML build errors
		}

		log.Printf("Built DPS XML for request %s, DPS ID: %s", requestID, dpsResult.DPSID)

		// Sign the DPS XML if certificate is provided
		dpsXML = dpsResult.XML
		if emissionReq.Certificate != nil && emissionReq.Certificate.HasCertificate && !emissionReq.Certificate.IsSigned {
			signedXML, signErr := p.signDPSXML(ctx, requestID, emissionReq.Certificate, dpsXML)
			if signErr != nil {
				// Signing error - don't retry
				rejectionInfo := &mongodb.RejectionInfo{
					Code:    emission.ErrorCodeCertificateError,
					Message: fmt.Sprintf("Failed to sign DPS XML: %v", signErr),
				}
				if updateErr := p.emissionRepo.UpdateRejection(ctx, requestID, rejectionInfo); updateErr != nil {
					log.Printf("Error updating rejection: %v", updateErr)
				}
				p.sendWebhook(ctx, emissionReq, nil, rejectionInfo)
				return nil // Don't retry signing errors
			}
			dpsXML = signedXML
			log.Printf("Signed DPS XML for request %s", requestID)
		} else if emissionReq.Certificate == nil || !emissionReq.Certificate.HasCertificate {
			log.Printf("No certificate provided for request %s, submitting unsigned DPS", requestID)
		} else {
			log.Printf("XML already signed for request %s, skipping signing step", requestID)
		}
	}

	// Submit to SEFIN
	environment := emissionReq.Environment
	if environment == "" {
		environment = sefin.EnvironmentHomologation
	}

	sefinResponse, err := p.sefinClient.SubmitDPS(ctx, dpsXML, environment)
	if err != nil {
		// Network/system error - retry
		if updateErr := p.emissionRepo.IncrementRetryCount(ctx, requestID, err.Error()); updateErr != nil {
			log.Printf("Error incrementing retry count: %v", updateErr)
		}
		return fmt.Errorf("SEFIN submission failed: %w", err)
	}

	// Process SEFIN response
	if sefinResponse.Success {
		// Success - update with result
		result := &mongodb.EmissionResult{
			NFSeAccessKey: sefinResponse.ChaveAcesso,
			NFSeNumber:    sefinResponse.NFSeNumber,
			NFSeXML:       sefinResponse.NFSeXML,
		}

		if err := p.emissionRepo.UpdateResult(ctx, requestID, result); err != nil {
			log.Printf("Error updating result: %v", err)
			return fmt.Errorf("failed to update result: %w", err)
		}

		log.Printf("Emission request %s completed successfully, NFS-e: %s", requestID, sefinResponse.NFSeNumber)
		p.sendWebhook(ctx, emissionReq, result, nil)
		return nil
	}

	// Rejection from SEFIN
	rejection := &mongodb.RejectionInfo{
		Code:           emission.ErrorCodeGovernmentRejection,
		Message:        sefinResponse.ErrorMessage,
		GovernmentCode: sefinResponse.ErrorCode,
	}

	if err := p.emissionRepo.UpdateRejection(ctx, requestID, rejection); err != nil {
		log.Printf("Error updating rejection: %v", err)
	}

	log.Printf("Emission request %s rejected by SEFIN: %s - %s", requestID, sefinResponse.ErrorCode, sefinResponse.ErrorMessage)
	p.sendWebhook(ctx, emissionReq, nil, rejection)

	// Don't retry government rejections
	return nil
}

// signDPSXML signs the DPS XML using the provided certificate.
func (p *EmissionProcessor) signDPSXML(ctx context.Context, requestID string, certData *mongodb.CertificateData, dpsXML string) (string, error) {
	if certData == nil || !certData.HasCertificate {
		return dpsXML, nil // Return unsigned if no certificate
	}

	// Parse the certificate
	certInfo, err := xmlsigner.ParsePFXBase64(certData.PFXBase64, certData.Password)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Validate the certificate for signing
	validator := xmlsigner.NewCertificateValidator()
	if err := validator.ValidateForSigning(certInfo); err != nil {
		return "", fmt.Errorf("certificate validation failed: %w", err)
	}

	// Create the signer and sign the DPS
	signer := xmlsigner.NewXMLSigner(certInfo)
	signedXML, err := signer.SignDPS(dpsXML)
	if err != nil {
		return "", fmt.Errorf("signing failed: %w", err)
	}

	// Update the signing status in the database (clear sensitive data)
	if updateErr := p.emissionRepo.UpdateSigningStatus(
		ctx,
		requestID,
		true,
		certInfo.GetSubjectCN(),
		certInfo.GetIssuerCN(),
		certInfo.GetSerialNumber(),
	); updateErr != nil {
		log.Printf("Warning: failed to update signing status: %v", updateErr)
		// Don't fail the operation, just log the warning
	}

	return signedXML, nil
}

// buildDPSXML creates the DPS XML document from the emission request.
func (p *EmissionProcessor) buildDPSXML(req *mongodb.EmissionRequest) (*xmlbuilder.DPSBuildResult, error) {
	// Determine environment code (1=production, 2=homologation)
	envCode := 2 // Default to homologation
	if req.Environment == "producao" || req.Environment == "production" {
		envCode = 1
	}

	config := xmlbuilder.DPSConfig{
		Environment:        envCode,
		EmissionDateTime:   time.Now(),
		ApplicationVersion: "1.0.0",
		Series:             req.DPS.Series,
		Number:             req.DPS.Number,
		CompetenceDate:     time.Now(),
		EmitterType:        1, // Service provider
		MunicipalityCode:   req.Service.MunicipalityCode,
		Substitution:       2, // No substitution
		Provider: xmlbuilder.DPSProvider{
			CNPJ:                  req.Provider.CNPJ,
			Name:                  req.Provider.Name,
			TaxRegime:             req.Provider.TaxRegime,
			MunicipalRegistration: req.Provider.MunicipalRegistration,
		},
		Service: xmlbuilder.DPSService{
			NationalCode:     req.Service.NationalCode,
			Description:      req.Service.Description,
			MunicipalityCode: req.Service.MunicipalityCode,
		},
		Values: xmlbuilder.DPSValues{
			ServiceValue:          req.Values.ServiceValue,
			UnconditionalDiscount: req.Values.UnconditionalDiscount,
			ConditionalDiscount:   req.Values.ConditionalDiscount,
			Deductions:            req.Values.Deductions,
		},
	}

	// Add taker if present
	if req.Taker != nil {
		config.Taker = &xmlbuilder.DPSTaker{
			CNPJ: req.Taker.CNPJ,
			CPF:  req.Taker.CPF,
			NIF:  req.Taker.NIF,
			Name: req.Taker.Name,
		}
	}

	builder := xmlbuilder.NewDPSBuilder(config)
	return builder.Build()
}

// sendWebhook sends a webhook notification for the emission result.
func (p *EmissionProcessor) sendWebhook(ctx context.Context, req *mongodb.EmissionRequest, result *mongodb.EmissionResult, rejection *mongodb.RejectionInfo) {
	// Skip if no webhook URL
	if req.WebhookURL == "" {
		log.Printf("No webhook URL configured for request %s", req.RequestID)
		return
	}

	// Determine event type and status
	var event, status string
	if result != nil {
		event = emission.WebhookEventEmissionCompleted
		status = emission.StatusSuccess
	} else {
		event = emission.WebhookEventEmissionFailed
		status = emission.StatusFailed
	}

	// Build payload
	payload := emission.WebhookPayload{
		Event:     event,
		RequestID: req.RequestID,
		Timestamp: time.Now().UTC(),
		Status:    status,
	}

	if result != nil {
		payload.Result = &emission.EmissionResultDTO{
			NFSeAccessKey: result.NFSeAccessKey,
			NFSeNumber:    result.NFSeNumber,
			NFSeXMLURL:    result.NFSeXMLURL,
		}
	}

	if rejection != nil {
		payload.Error = &emission.EmissionErrorDTO{
			Code:           rejection.Code,
			Message:        rejection.Message,
			GovernmentCode: rejection.GovernmentCode,
			Details:        rejection.Details,
		}
	}

	// Create webhook delivery record
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling webhook payload: %v", err)
		return
	}

	delivery := &mongodb.WebhookDelivery{
		RequestID: req.RequestID,
		APIKeyID:  req.APIKeyID,
		URL:       req.WebhookURL,
		Status:    mongodb.WebhookStatusPending,
		Payload:   string(payloadBytes),
	}

	if err := p.webhookRepo.Create(ctx, delivery); err != nil {
		log.Printf("Error creating webhook delivery record: %v", err)
		return
	}

	// Send webhook (TODO: Get secret from API key)
	webhookSecret := "" // In production, get this from the API key
	sendResult, err := p.webhookSender.Send(ctx, req.WebhookURL, payload, webhookSecret, req.RequestID)
	if err != nil {
		log.Printf("Webhook delivery failed for request %s: %v", req.RequestID, err)
		if markErr := p.webhookRepo.MarkFailed(ctx, delivery.ID, err.Error(), sendResult.Duration.Milliseconds()); markErr != nil {
			log.Printf("Error marking webhook as failed: %v", markErr)
		}
		return
	}

	if sendResult.Success {
		log.Printf("Webhook delivered successfully for request %s", req.RequestID)
		if markErr := p.webhookRepo.MarkSuccess(ctx, delivery.ID, sendResult.StatusCode, sendResult.ResponseBody, sendResult.Duration.Milliseconds()); markErr != nil {
			log.Printf("Error marking webhook as success: %v", markErr)
		}
	} else {
		log.Printf("Webhook delivery failed for request %s: %s", req.RequestID, sendResult.Error)
		if markErr := p.webhookRepo.MarkFailed(ctx, delivery.ID, sendResult.Error, sendResult.Duration.Milliseconds()); markErr != nil {
			log.Printf("Error marking webhook as failed: %v", markErr)
		}
	}
}

// WebhookProcessor handles webhook delivery tasks.
type WebhookProcessor struct {
	webhookRepo   *mongodb.WebhookRepository
	webhookSender *webhook.Sender
	apiKeyRepo    *mongodb.APIKeyRepository
}

// WebhookProcessorConfig configures the webhook processor.
type WebhookProcessorConfig struct {
	WebhookRepo   *mongodb.WebhookRepository
	WebhookSender *webhook.Sender
	APIKeyRepo    *mongodb.APIKeyRepository
}

// NewWebhookProcessor creates a new webhook processor.
func NewWebhookProcessor(config WebhookProcessorConfig) *WebhookProcessor {
	return &WebhookProcessor{
		webhookRepo:   config.WebhookRepo,
		webhookSender: config.WebhookSender,
		apiKeyRepo:    config.APIKeyRepo,
	}
}

// ProcessWebhook handles the webhook:delivery task.
func (p *WebhookProcessor) ProcessWebhook(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseWebhookTask(task)
	if err != nil {
		log.Printf("Error parsing webhook task: %v", err)
		return nil // Don't retry invalid payloads
	}

	log.Printf("Processing webhook delivery: %s", payload.DeliveryID)

	// This would be implemented for retry logic in a production system
	// For now, webhooks are sent synchronously in the emission processor

	return nil
}
