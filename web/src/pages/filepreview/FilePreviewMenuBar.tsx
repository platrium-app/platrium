import React from "react";
import {
    Menubar,
    MenubarContent,
    MenubarItem,
    MenubarMenu,
    MenubarSeparator,
    MenubarShortcut,
    MenubarTrigger,
} from "@/components/ui/menubar";
import { Download, Info, X, ScanBarcode, QrCode } from "lucide-react";
import { Button } from "@/components/ui/button";
import { triggerFileDownload } from "@/lib/utils";
import { getFileIcon } from "@/lib/fileIcons";
import type { FilePreviewInfo, PluginMenuItem, PreviewFeatures } from "./PluginDefinition";

import { useFilePreviewMenuContext, useRegisterMenu } from "./FilePreviewMenuContext";

interface FilePreviewMenuBarProps {
    info?: FilePreviewInfo | null;
    features?: PreviewFeatures;
    isModal?: boolean;
    onClose?: () => void;
}

const CATEGORY_ORDER = ["File", "Edit", "View", "Tools"];

const getCategoryIndex = (category: string) => {
    const idx = CATEGORY_ORDER.indexOf(category);
    return idx !== -1 ? idx : CATEGORY_ORDER.length;
};

export const FilePreviewMenuBar: React.FC<FilePreviewMenuBarProps> = ({
    info,
    isModal,
    onClose,
}) => {
    const menuContext = useFilePreviewMenuContext();

    // Core default items registered into Context using the unified registration hook
    useRegisterMenu("File", [
        {
            id: "download",
            label: "Download",
            category: "File",
            icon: Download,
            isDefault: true,
            onClick: (f) => f && triggerFileDownload(f.fileId, f.fileName),
        },
        {
            id: "file-info",
            label: "File Info",
            category: "File",
            icon: Info,
            isDefault: true,
            onClick: () => {
                /* info action */
            },
        },
    ]);

    useRegisterMenu("Tools", [
        {
            id: "copy-file-id",
            label: "Copy File ID",
            category: "Tools",
            icon: ScanBarcode,
            isDefault: true,
            onClick: (f) => f && navigator.clipboard?.writeText(f.fileId),
        },
        {
            id: "show-qr-code",
            label: "Show QR Code",
            category: "Tools",
            icon: QrCode,
            isDefault: true,
            onClick: () => {
                /* QR code action */
            },
        },
    ]);

    const sortedMenuEntries = React.useMemo(() => {
        const groups = new Map<string, PluginMenuItem[]>();

        const contextItems: PluginMenuItem[] = menuContext
            ? Object.values(menuContext.registeredItems).flat()
            : [];

        contextItems.forEach((item) => {
            const list = groups.get(item.category) || [];
            list.push(item);
            groups.set(item.category, list);
        });

        for (const [category, list] of groups.entries()) {
            const customItems = list.filter((item) => !item.isDefault);
            const defaultItems = list.filter((item) => item.isDefault);
            
            if (customItems.length > 0 && defaultItems.length > 0) {
                // Ensure the first default item has a separator to split custom from default
                defaultItems[0] = { ...defaultItems[0], separatorBefore: true };
            }
            
            groups.set(category, [...customItems, ...defaultItems]);
        }

        return Array.from(groups.entries()).sort(
            ([catA], [catB]) => getCategoryIndex(catA) - getCategoryIndex(catB)
        );
    }, [menuContext]);

    return (
        <div className="h-10 border-b border-border bg-background/95 backdrop-blur-md shadow-xs z-10 relative flex items-center justify-between px-3 select-none shrink-0">
            {/* Left: File Icon + File Name + Vertical Separator + Menubar */}
            <div className="flex items-center space-x-2.5 min-w-0">
                {info ? (
                    <>
                        <div className="flex items-center space-x-2 min-w-0">
                            {getFileIcon(info.fileName, info.mimeType, "h-4 w-4 shrink-0")}
                            <span
                                className="text-xs font-semibold text-foreground truncate max-w-[200px]"
                                title={info.fileName}
                            >
                                {info.fileName}
                            </span>
                        </div>

                        <div className="h-4 w-px bg-border/60 mx-0.5 shrink-0" />

                        {/* Unified Menubar Renderer */}
                        <Menubar className="border-none bg-transparent h-7 p-0 gap-0.5">
                            {sortedMenuEntries.map(([category, items]) => (
                                <MenubarMenu key={category}>
                                    <MenubarTrigger className="h-7 px-2.5 text-xs font-medium">{category}</MenubarTrigger>
                                    <MenubarContent>
                                        {items.map((item) => {
                                            const IconComponent = item.icon;
                                            return (
                                                <React.Fragment key={item.id}>
                                                    {item.separatorBefore && <MenubarSeparator />}
                                                    <MenubarItem
                                                        disabled={item.disabled}
                                                        onClick={() => item.onClick(info)}
                                                    >
                                                        {IconComponent && <IconComponent className="mr-2 h-4 w-4" />}
                                                        <span>{item.label}</span>
                                                        {item.shortcut && <MenubarShortcut>{item.shortcut}</MenubarShortcut>}
                                                    </MenubarItem>
                                                </React.Fragment>
                                            );
                                        })}
                                    </MenubarContent>
                                </MenubarMenu>
                            ))}
                        </Menubar>
                    </>
                ) : (
                    <div className="flex items-center space-x-2">
                        <div className="h-4 w-4 rounded bg-muted/60 animate-pulse shrink-0" />
                        <div className="h-3 w-32 rounded bg-muted/60 animate-pulse" />
                    </div>
                )}
            </div>

            {/* Right side controls: Modal Close button only */}
            {isModal && onClose && (
                <div className="flex items-center shrink-0">
                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose} title="Close Preview">
                        <X className="h-3.5 w-3.5" />
                    </Button>
                </div>
            )}
        </div>
    );
};

export default FilePreviewMenuBar;
