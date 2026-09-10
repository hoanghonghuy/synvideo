import { createI18n } from 'vue-i18n'

import vi from './vi'

const viMessages = {
  ...vi,
  projects: {
    ...vi.projects,
    detail: {
      ...vi.projects.detail,
      sceneVideoAction: 'Tạo video phân cảnh',
    },
  },
} as const

export const datetimeFormats = {
  vi: {
    long: {
      year: 'numeric',
      month: 'numeric',
      day: 'numeric',
      hour: 'numeric',
      minute: 'numeric',
      second: 'numeric',
    },
  },
  en: {
    long: {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
      hour: 'numeric',
      minute: 'numeric',
      second: 'numeric',
    },
  },
} as const

export const i18n = createI18n({
  legacy: false,
  locale: 'vi',
  fallbackLocale: 'vi',
  messages: {
    vi: viMessages,
  },
  datetimeFormats,
})
