export default {
  app: {
    name: 'SynVideo',
  },
  navigation: {
    primary: 'Điều hướng chính',
    home: 'Trang chủ',
    status: 'Trạng thái',
    projects: 'Dự án',
  },
  home: {
    eyebrow: 'Nền tảng kỹ thuật',
    title: 'Không gian làm video AI bắt đầu từ một nền tảng rõ ràng.',
    description:
      'Ứng dụng web Vue đã sẵn sàng cho routing, TypeScript và tài nguyên ngôn ngữ tiếng Việt.',
    projectsAction: 'Mở danh sách dự án',
  },
  status: {
    eyebrow: 'Kiểm tra hệ thống',
    title: 'Các ranh giới ứng dụng đã được thiết lập.',
    frontend: 'Frontend',
    router: 'Router',
    localization: 'i18n tiếng Việt',
    ready: 'Sẵn sàng',
  },
  projects: {
    list: {
      eyebrow: 'Dự án',
      title: 'Dự án của bạn',
      empty: 'Chưa có dự án nào được tạo.',
    },
    create: {
      eyebrow: 'Dự án mới',
      title: 'Tạo dự án',
    },
    detail: {
      eyebrow: 'Chi tiết dự án',
      updatedAt: 'Cập nhật lần cuối: {value}',
    },
    fields: {
      title: 'Tiêu đề',
      description: 'Mô tả',
      contentFormat: 'Định dạng nội dung',
      aspectRatio: 'Tỷ lệ khung hình',
      duration: 'Thời lượng mục tiêu, giây',
      locale: 'Ngôn ngữ nội dung',
      status: 'Trạng thái',
    },
    contentFormat: {
      short: 'Ngắn',
      long: 'Dài',
      flexible: 'Linh hoạt',
    },
    locale: {
      vi: 'Tiếng Việt',
      en: 'Tiếng Anh',
    },
    status: {
      active: 'Đang hoạt động',
      archived: 'Đã lưu trữ',
    },
    actions: {
      create: 'Tạo dự án',
      saveCreate: 'Lưu dự án',
      saveUpdate: 'Lưu thay đổi',
      submitting: 'Đang lưu',
      retry: 'Thử lại',
      loadMore: 'Tải thêm',
      backToList: 'Quay lại danh sách',
    },
    states: {
      loading: 'Đang tải',
      saved: 'Đã lưu thay đổi.',
    },
    errors: {
      request_failed: 'Không thể kết nối máy chủ.',
      validation_failed: 'Vui lòng kiểm tra lại thông tin dự án.',
      project_not_found: 'Không tìm thấy dự án.',
      principal_required: 'Chưa có định danh người dùng cục bộ cho môi trường này.',
      internal_error: 'Máy chủ chưa thể hoàn tất yêu cầu.',
    },
    validation: {
      required: 'Bắt buộc nhập.',
      max_160: 'Tối đa 160 ký tự.',
      max_5000: 'Tối đa 5000 ký tự.',
      invalid: 'Giá trị không hợp lệ.',
      invalid_json: 'Dữ liệu gửi lên không hợp lệ.',
      range_1_43200: 'Nhập từ 1 đến 43200 giây.',
      range_1_100: 'Nhập từ 1 đến 100.',
    },
  },
} as const
