// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

import { parseISO, intlFormatDistance } from 'date-fns'

export function FormatTimestamp(object) {
  const absoluteTime = object.getAttribute('datetime')
  let dateValue = parseISO(absoluteTime, new Date())
  const timeString = intlFormatDistance(dateValue, new Date())
  object.textContent = timeString
}

window.customElements.define(
  'human-time',
  class extends HTMLElement {
    static observedAttributes = ['datetime']

    alive = false
    contentSpan = null

    update = (recurring) => {
      if (!this.alive) return

      if (!this.shadowRoot) {
        this.attachShadow({ mode: 'open' })
        this.contentSpan = document.createElement('span')
        this.shadowRoot.append(this.contentSpan)
      }

      const next = FormatTimestamp(this)
      if (recurring && next !== null)
        setTimeout(() => {
          this.update(true)
        }, next)
    }

    connectedCallback() {
      this.alive = true
      this.update(true)
    }

    disconnectedCallback() {
      this.alive = false
    }

    attributeChangedCallback(name, oldValue, newValue) {
      if (name === 'datetime' && oldValue !== newValue) this.update(false)
    }

    set textContent(value) {
      if (this.contentSpan) this.contentSpan.textContent = value
    }
    get textContent() {
      return this.contentSpan?.textContent
    }
  }
)
