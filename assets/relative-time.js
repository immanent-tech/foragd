import { GetPluralizedString } from './i18n.js'
import dayjs from 'dayjs'
const { pageData } = window.config

export const HALF_MINUTE = 30 * 1000
export const ONE_MINUTE = 60 * 1000
export const ONE_HOUR = 60 * ONE_MINUTE
export const ONE_DAY = 24 * ONE_HOUR

const ABSOLUTE_DATETIME_FORMAT = new Intl.DateTimeFormat(navigator.language, {
  year: 'numeric',
  month: 'short',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  timeZoneName: 'short',
})
const FALLBACK_DATETIME_FORMAT = new Intl.RelativeTimeFormat(
  navigator.language,
  { style: 'long' }
)

function GetPluralizedStringOrFallback(key, n, unit) {
  const translation = GetPluralizedString(key, n, true)
  if (translation) return translation.replace('%d', n)
  return FALLBACK_DATETIME_FORMAT.format(-n, unit)
}

/**
 * Update the displayed text of the given relative-time DOM element with its
 * human-readable, localized relative time string.
 * Returns the recommended interval in milliseconds until the object should be updated again,
 * or null if the object is invalid.
 */
export function DoUpdateRelativeTime(object, now) {
  const absoluteTime = object.getAttribute('datetime')
  if (!absoluteTime) {
    return null // Object does not contain a datetime.
  }

  if (!now) now = Date.now()

  const nowJS = dayjs(now)
  const thenJS = dayjs(absoluteTime)

  object.setAttribute(
    'data-tooltip-content',
    ABSOLUTE_DATETIME_FORMAT.format(thenJS.toDate())
  )

  if (nowJS.isBefore(thenJS)) {
    // Datetime is in the future.
    object.textContent = pageData.DATETIMESTRINGS.FUTURE
    return -Math.floor(nowJS.diff(thenJS, 'millisecond'))
  }

  const years = Math.floor(nowJS.diff(thenJS, 'year'))
  if (years >= 1) {
    // Datetime is at least one year ago.
    if (years === 1 && pageData.DATETIMESTRINGS['relativetime.1year']) {
      // Datetime is one year ago.
      object.textContent = pageData.DATETIMESTRINGS['relativetime.1year']
    } else if (years === 2 && pageData.DATETIMESTRINGS['relativetime.2years']) {
      // Datetime is two year ago.
      object.textContent = pageData.DATETIMESTRINGS['relativetime.2years']
    } else {
      // Datetime is more than a year ago.
      object.textContent = GetPluralizedStringOrFallback(
        'relativetime.years',
        years,
        'year'
      )
    }
    return ONE_DAY
  }

  const months = Math.floor(nowJS.diff(thenJS, 'month'))
  if (months >= 1) {
    // Datetime is at least one month but less than a year ago.
    if (months === 1 && pageData.DATETIMESTRINGS['relativetime.1month']) {
      // Datetime is one month ago.
      object.textContent = pageData.DATETIMESTRINGS['relativetime.1month']
    } else if (
      months === 2 &&
      pageData.DATETIMESTRINGS['relativetime.2months']
    ) {
      // Datetime is two months ago.
      object.textContent = pageData.DATETIMESTRINGS['relativetime.2months']
    } else {
      // Datetime is several months ago (but less than a year).
      object.textContent = GetPluralizedStringOrFallback(
        'relativetime.months',
        months,
        'month'
      )
    }
    return ONE_DAY
  }

  const weeks = Math.floor(nowJS.diff(thenJS, 'week'))
  if (weeks >= 1) {
    // Datetime is at least one week but less than a month ago.
    if (weeks === 1 && pageData.DATETIMESTRINGS['relativetime.1week']) {
      // Datetime is one week ago.
      object.textContent = pageData.DATETIMESTRINGS['relativetime.1week']
    } else if (weeks === 2 && pageData.DATETIMESTRINGS['relativetime.2weeks']) {
      // Datetime is two week ago.
      object.textContent = pageData.DATETIMESTRINGS['relativetime.2weeks']
    } else {
      // Datetime is several weeks ago (but less than a month).
      object.textContent = GetPluralizedStringOrFallback(
        'relativetime.weeks',
        weeks,
        'week'
      )
    }
    return ONE_DAY
  }

  const days = Math.floor(nowJS.diff(thenJS, 'day'))
  if (days >= 1) {
    if (days === 1 && pageData.DATETIMESTRINGS['relativetime.1day']) {
      // Datetime is one day ago.
      object.textContent = pageData.DATETIMESTRINGS['relativetime.1day']
    } else if (days === 2 && pageData.DATETIMESTRINGS['relativetime.2days']) {
      // Datetime is two days ago.
      object.textContent = pageData.DATETIMESTRINGS['relativetime.2days']
    } else {
      // Datetime is several days but less than a week ago.
      object.textContent = GetPluralizedStringOrFallback(
        'relativetime.days',
        days,
        'day'
      )
    }
    return ONE_DAY
  }

  const hours = Math.floor(nowJS.diff(thenJS, 'hour'))
  if (hours >= 1) {
    // Datetime is one or more hours but less than a day ago.
    object.textContent = GetPluralizedStringOrFallback(
      'relativetime.hours',
      hours,
      'hour'
    )
    return ONE_HOUR
  }

  const minutes = Math.floor(nowJS.diff(thenJS, 'minute'))
  if (minutes >= 1) {
    // Datetime is one or more minutes but less than an hour ago.
    object.textContent = GetPluralizedStringOrFallback(
      'relativetime.mins',
      minutes,
      'minute'
    )
    return ONE_MINUTE
  }

  // Datetime is very recent.
  object.textContent = pageData.DATETIMESTRINGS.NOW
  return HALF_MINUTE
}

window.customElements.define(
  'relative-time',
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

      const next = DoUpdateRelativeTime(this)
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
