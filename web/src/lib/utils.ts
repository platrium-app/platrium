import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function constructRawContentUrl(fileId: string): string {
  return `/rawcontent/${fileId}`
}

export function triggerFileDownload(fileId: string, fileName?: string): void {
  const url = `${constructRawContentUrl(fileId)}?dl=1`
  const link = document.createElement("a")
  link.href = url
  if (fileName) {
    link.download = fileName
  }
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

export function parseFilenameFromContentDisposition(disposition?: string | null, fallback = ""): string {
  if (!disposition) return fallback

  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match) {
    return decodeURIComponent(utf8Match[1])
  }

  const asciiMatch = disposition.match(/filename="([^"]+)"/i)
  if (asciiMatch) {
    return asciiMatch[1]
  }

  return fallback
}
