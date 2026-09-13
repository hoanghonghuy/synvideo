import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
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
    expect(wrapper.find('#project-title-error').exists()).toBe(true)

    const duration = wrapper.get('input[name="duration"]')
    expect(duration.attributes('aria-invalid')).toBe('true')
    expect(duration.attributes('aria-describedby')).toBe('project-duration-error')
    expect(wrapper.find('#project-duration-error').exists()).toBe(true)
  })

  it('focuses the first invalid control in form order when server field errors arrive', async () => {
    const wrapper = mount(ProjectForm, {
      attachTo: document.body,
      props: {
        submitting: false,
        submitLabel: 'Save',
        fieldErrors: {},
      },
      global: { plugins: [i18n] },
    })

    await wrapper.setProps({
      fieldErrors: {
        target_duration_seconds: 'invalid',
        title: 'required',
      },
    })
    // The component intentionally waits for the error DOM to render before focusing.
    // Let that post-render focus effect finish before asserting activeElement.
    await nextTick()

    const title = wrapper.get('input[name="title"]')
    const duration = wrapper.get('input[name="duration"]')
    expect(title.attributes('aria-invalid')).toBe('true')
    expect(duration.attributes('aria-invalid')).toBe('true')
    expect(document.activeElement).toBe(title.element)

    wrapper.unmount()
  })

  it('does not move focus when server field errors are cleared', async () => {
    const wrapper = mount(ProjectForm, {
      attachTo: document.body,
      props: {
        submitting: false,
        submitLabel: 'Save',
        fieldErrors: { title: 'required' },
      },
      global: { plugins: [i18n] },
    })

    const submit = wrapper.get('button[type="submit"]')
    ;(submit.element as HTMLButtonElement).focus()
    await wrapper.setProps({ fieldErrors: {} })

    expect(document.activeElement).toBe(submit.element)
    wrapper.unmount()
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
