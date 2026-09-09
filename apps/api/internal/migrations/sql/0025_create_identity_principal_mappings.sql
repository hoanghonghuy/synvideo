CREATE TABLE identity_principal_mappings (
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL,
    owner_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (issuer, subject)
);

CREATE INDEX identity_principal_mappings_owner_id_idx ON identity_principal_mappings (owner_id);
