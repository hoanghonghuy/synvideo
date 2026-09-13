import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import { i18n } from '@/locales'
import ProjectForm from './ProjectForm.vue'

describe('ProjectForm accessibility', () => {
  it('associates server field errors with the exact controls', () => {
    const wrapper = mount(ProjectForm, {
      props: {
        submitting: false,
        submitLabel: 'Save',
        fieldErrors: {
          title: 'required',
          target_duration_seconds: 'invalid',
        },
      },
      global: { plugins: [i18n] },
    })

    const title = wrapper.get('input[name="title"]')
    expect(title.attributes('aria-invalid')).toBe('true')
    expect(title.attributes('aria-describedby')).toBe('project-title-error')
    expect(wrapper.get('#project-title-error').exists()).toBe(true)

    const duration = wrapper.get('input[name="duration"]')
    expect(duration.attributes('aria-invalid')).toBe('true')
    expect(duration.attributes('aria-describedby')).toBe('project-duration-error')
    expect(wrapper.get('#project-duration-error').exists()).toBe(true)
  })

  it('focuses invalid duration and does not submit', async () => {
    const wrapper = mount(ProjectForm, {
      attachTo: document.body,
      props: {
        submitting: false,
        submitLabel: 'Save',
      },
      global: { plugins: [i18n] },
    })

    const duration = wrapper.get('input[name="duration"]')
    await duration.setValue('12.5')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('submit')).toBeUndefined()
    expect(duration.attributes('aria-invalid')).toBe('true')
    expect(duration.attributes('aria-describedby')).toBe('project-duration-error')
    expect(document.activeElement).toBe(duration.element)

    wrapper.unmount()
  })
})
