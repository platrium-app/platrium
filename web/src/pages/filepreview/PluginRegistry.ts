import { type PluginDefinition } from './PluginDefinition';
import { FallbackPlugin } from './plugins/FallbackPlugin';
import { ImagePreviewPlugin } from './plugins/ImagePreviewPlugin';
import { PdfPreviewPlugin } from './plugins/PdfPreviewPlugin';
import { VideoPreviewPlugin } from './plugins/VideoPreviewPlugin';
import { AudioPreviewPlugin } from './plugins/AudioPreviewPlugin';

class PluginRegistry {
  private plugins: PluginDefinition[] = [];
  // Cache evaluated mime-types to O(1) lookups
  private mimeTypeCache: Map<string, PluginDefinition> = new Map();

  register(plugin: PluginDefinition) {
    this.plugins.push(plugin);
  }

  getPluginForMimeType(mimeType: string): PluginDefinition {
    if (this.mimeTypeCache.has(mimeType)) {
      return this.mimeTypeCache.get(mimeType)!;
    }

    // Find the first plugin that supports the mimetype (handling exact match or wildcards like image/*)
    const plugin = this.plugins.find(p =>
      p.supportedMimeTypes.some(supported =>
        supported === mimeType ||
        (supported.endsWith('/*') && mimeType.startsWith(supported.replace('/*', '')))
      )
    ) || FallbackPlugin;

    this.mimeTypeCache.set(mimeType, plugin);
    return plugin;
  }
}

export const previewRegistry = new PluginRegistry();
previewRegistry.register(ImagePreviewPlugin);
previewRegistry.register(PdfPreviewPlugin);
previewRegistry.register(VideoPreviewPlugin);
previewRegistry.register(AudioPreviewPlugin);
