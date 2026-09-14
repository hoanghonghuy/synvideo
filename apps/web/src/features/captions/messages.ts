export default {
  vi: { captions: {
    eyebrow: 'TASK-034 · Phụ đề', title: 'Không gian căn thời gian phụ đề', description: 'Phụ đề được khóa với exact narration/audio lineage và thời lượng audio đã đo.', back: 'Quay lại dự án',
    planVersion: 'Phiên bản scene plan', sceneKey: 'Scene key', load: 'Tải', derive: 'Tạo lần đầu',
    conflict: 'Bản phụ đề đã thay đổi ở nơi khác. Hãy tải lại trước khi lưu.', sourceMissing: 'Cảnh này chưa có narration audio hợp lệ với thời lượng đo được.', staleError: 'Phụ đề đang stale. Hãy rebuild có chủ đích trước khi dùng làm snapshot hiện hành.', notFound: 'Chưa có phụ đề cho cảnh này.', requestFailed: 'Không thể hoàn thành yêu cầu.',
    derived: 'Đã tạo phụ đề từ exact narration lineage hiện tại.', savedStale: 'Đã lưu chỉnh sửa. Phụ đề vẫn stale vì source narration đã thay đổi.', saved: 'Đã lưu phụ đề.', rebuilt: 'Đã rebuild sang narration lineage hiện tại. Revision cũ vẫn được giữ trong lịch sử.',
    revision: 'revision {revision}', sourceDuration: 'source {duration} ms', rebuild: 'Rebuild từ narration hiện tại', segments: 'Các đoạn', segmentHelp: 'Không overlap; 0 ≤ start < end ≤ source duration.', addSegment: 'Thêm đoạn', text: 'Nội dung', start: 'Bắt đầu (ms)', end: 'Kết thúc (ms)', remove: 'Xóa',
    style: 'Kiểu hiển thị độc lập render', alignment: 'Căn chỉnh', position: 'Vị trí', size: 'Kích thước', weight: 'Độ đậm', fontToken: 'Font token', optionalToken: 'token tùy chọn', saveRevision: 'Lưu revision mới',
    history: 'Lịch sử revision', noHistory: 'Chưa có lịch sử.', historyItem: 'Revision {revision} · {duration} ms · source {source}…'
  }},
  en: { captions: {
    eyebrow: 'TASK-034 · Captions', title: 'Caption timing workspace', description: 'Captions are locked to the exact narration/audio lineage and measured audio duration.', back: 'Back to project',
    planVersion: 'Scene plan version', sceneKey: 'Scene key', load: 'Load', derive: 'Derive initial captions',
    conflict: 'The caption revision changed elsewhere. Reload before saving.', sourceMissing: 'This scene has no valid narration audio with a measured duration.', staleError: 'Captions are stale. Rebuild intentionally before using them in the current snapshot.', notFound: 'No captions exist for this scene.', requestFailed: 'Could not complete the request.',
    derived: 'Captions derived from the current exact narration lineage.', savedStale: 'Changes saved. Captions remain stale because source narration changed.', saved: 'Captions saved.', rebuilt: 'Rebuilt against the current narration lineage. The previous revision remains in history.',
    revision: 'revision {revision}', sourceDuration: 'source {duration} ms', rebuild: 'Rebuild from current narration', segments: 'Segments', segmentHelp: 'No overlap; 0 ≤ start < end ≤ source duration.', addSegment: 'Add segment', text: 'Text', start: 'Start (ms)', end: 'End (ms)', remove: 'Remove',
    style: 'Render-neutral style', alignment: 'Alignment', position: 'Position', size: 'Size', weight: 'Weight', fontToken: 'Font token', optionalToken: 'optional token', saveRevision: 'Save new revision',
    history: 'Revision history', noHistory: 'No history yet.', historyItem: 'Revision {revision} · {duration} ms · source {source}…'
  }}
} as const
