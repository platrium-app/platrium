import { type ComponentType } from "react";

export interface FilePreviewInfo {
  fileId: string;
  fileName: string;
  mimeType: string;
  sizeBytes?: number;
}

export type MenuCategory = "File" | "Edit" | "View" | "Tools" | (string & {});

export interface PluginMenuItem {
  id: string;
  label: string;
  category: MenuCategory;
  icon?: ComponentType<{ className?: string }>;
  disabled?: boolean;
  checked?: boolean;
  separatorBefore?: boolean;
  shortcut?: string;
  isDefault?: boolean;
  onClick: (info: FilePreviewInfo) => void;
}

// Features that a plugin can register with the Core component
export interface PreviewFeatures {
  // Reserved for plugin-to-core state signals if needed
  [key: string]: unknown;
}

export interface FilePreviewPluginProps {
  info: FilePreviewInfo;

  // Callback used by the plugin to tell Core what controls & menus to render
  registerFeatures: (features: PreviewFeatures) => void;

  title?: string;
  description?: string;

  onError?: (error: Error) => void;
  onLoaded: () => void;
}

export const DEFAULT_MAX_PREVIEW_SIZE_BYTES = 32 * 1024 * 1024; // 32 MB default limit

export interface PluginDefinition {
  id: string;
  name: string;
  // Exact mime-types or glob patterns (e.g. "image/*") this plugin supports.
  // Leave empty for a fallback plugin.
  supportedMimeTypes: string[];
  // Maximum file size in bytes for previewing.
  // undefined = defaults to 32MB (DEFAULT_MAX_PREVIEW_SIZE_BYTES)
  // 0 = unlimited (e.g. progressive video / range-based streaming)
  maxSizeBytes?: number;
  component: ComponentType<FilePreviewPluginProps>;
}
