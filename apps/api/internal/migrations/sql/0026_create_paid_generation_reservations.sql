CREATE TABLE paid_generation_reservations (
    owner_id UUID NOT NULL,
    project_id UUID NOT NULL,
    operation_kind TEXT NOT NULL,
    request_id UUID NOT NULL,
    state TEXT NOT NULL DEFAULT 'reserved',
    reserved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    released_at TIMESTAMPTZ,
    PRIMARY KEY (owner_id, project_id, operation_kind, request_id),
    CONSTRAINT paid_generation_reservations_operation_check
        CHECK (operation_kind IN ('image', 'narration', 'video')),
    CONSTRAINT paid_generation_reservations_state_check
        CHECK (state IN ('reserved', 'released'))
);

CREATE INDEX paid_generation_reservations_window_idx
    ON paid_generation_reservations (owner_id, project_id, operation_kind, reserved_at DESC);

CREATE INDEX paid_generation_reservations_inflight_idx
    ON paid_generation_reservations (owner_id, project_id, operation_kind)
    WHERE state = 'reserved';
