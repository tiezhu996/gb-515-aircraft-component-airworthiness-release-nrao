package model

import "time"

// CertificateRecord models 证书记录 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type CertificateRecord struct {
	BaseModel
	Facility    string                      `json:"facility" gorm:"size:120;index"`
	Owner       string                      `json:"owner" gorm:"size:120;index"`
	Category    string                      `json:"category" gorm:"size:80;index"`
	RiskLevel   string                      `json:"riskLevel" gorm:"size:32;index"`
	MetricValue float64                     `json:"metricValue"`
	MetricUnit  string                      `json:"metricUnit" gorm:"size:24"`
	EffectiveAt time.Time                   `json:"effectiveAt"`
	Evidence    string                      `json:"evidence" gorm:"size:2000"`
	RelatedCode string                      `json:"relatedCode" gorm:"size:64;index"`
	PreparedBy  string                      `json:"preparedBy" gorm:"size:80;index"`
	VerifiedBy  string                      `json:"verifiedBy" gorm:"size:80;index"`
	Revisions   []CertificateRecordRevision `json:"revisions,omitempty" gorm:"foreignKey:CertificateRecordID"`
}

func (item *CertificateRecord) GetBase() *BaseModel { return &item.BaseModel }

func (item CertificateRecord) TableName() string { return "certificate_records" }

var CertificateRecordInitialStatus = "draft"

// CertificateRecordRevision is append-only evidence of every certificate
// version. Actor and request ID make each published snapshot attributable.
type CertificateRecordRevision struct {
	ID                  uint      `json:"id" gorm:"primaryKey"`
	CertificateRecordID uint      `json:"certificateRecordId" gorm:"not null;index;uniqueIndex:idx_certificate_revision_version,priority:1"`
	Version             uint      `json:"version" gorm:"not null;uniqueIndex:idx_certificate_revision_version,priority:2"`
	Status              string    `json:"status" gorm:"size:40;not null"`
	Evidence            string    `json:"evidence" gorm:"size:2000"`
	Actor               string    `json:"actor" gorm:"size:80;not null;index"`
	RequestID           string    `json:"requestId" gorm:"size:64;not null;index"`
	Action              string    `json:"action" gorm:"size:40;not null"`
	Reason              string    `json:"reason" gorm:"size:500"`
	CreatedAt           time.Time `json:"createdAt" gorm:"index"`
}
