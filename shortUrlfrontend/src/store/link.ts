import { defineStore } from 'pinia'

export interface ShortLinkRecord {
  longURL: string
  shortURL: string
  category?: string
  safetyStatus?: string
  aiSuggestions?: string[]
}

export const useLinkStore = defineStore('link', {
  state: () => ({
    links: [] as ShortLinkRecord[]
  }),
  actions: {
    addLink(item: ShortLinkRecord) {
      this.links.unshift(item)
    }
  }
})
