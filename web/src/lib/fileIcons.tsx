import {
  FileIcon,
  FileText,
  FileImage,
  FileVideo,
  FileAudio,
  FileCode,
  FileArchive,
  FileSpreadsheet,
} from "lucide-react";

export function getFileIcon(fileName?: string, mimeType?: string, className = "h-4 w-4 shrink-0") {
  const ext = fileName?.split(".").pop()?.toLowerCase() || "";
  const mime = mimeType?.toLowerCase() || "";

  if (mime.startsWith("image/") || ["png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico"].includes(ext)) {
    return <FileImage className={`${className} text-blue-400`} />;
  }
  if (mime.startsWith("video/") || ["mp4", "webm", "mkv", "mov", "avi"].includes(ext)) {
    return <FileVideo className={`${className} text-rose-400`} />;
  }
  if (mime.startsWith("audio/") || ["mp3", "wav", "ogg", "flac", "m4a"].includes(ext)) {
    return <FileAudio className={`${className} text-amber-400`} />;
  }
  if (
    mime.includes("json") ||
    mime.includes("javascript") ||
    mime.includes("typescript") ||
    mime.includes("xml") ||
    mime.includes("html") ||
    ["js", "ts", "tsx", "jsx", "rs", "go", "py", "c", "cpp", "h", "html", "css", "json", "yaml", "yml", "toml"].includes(ext)
  ) {
    return <FileCode className={`${className} text-amber-400`} />;
  }
  if (
    mime.includes("zip") ||
    mime.includes("archive") ||
    mime.includes("compressed") ||
    ["zip", "tar", "gz", "rar", "7z", "bz2"].includes(ext)
  ) {
    return <FileArchive className={`${className} text-purple-400`} />;
  }
  if (
    mime.includes("spreadsheet") ||
    mime.includes("excel") ||
    mime.includes("csv") ||
    ["csv", "xls", "xlsx"].includes(ext)
  ) {
    return <FileSpreadsheet className={`${className} text-emerald-400`} />;
  }
  if (
    mime.includes("pdf") ||
    mime.includes("document") ||
    mime.includes("text") ||
    ["pdf", "doc", "docx", "txt", "md", "rtf"].includes(ext)
  ) {
    return <FileText className={`${className} text-emerald-400`} />;
  }

  return <FileIcon className={`${className} text-muted-foreground`} />;
}
