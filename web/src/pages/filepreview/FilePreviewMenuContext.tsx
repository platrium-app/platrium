import React, { createContext, useContext, useState, useEffect, useId, useCallback, useMemo, useRef } from "react";
import type { PluginMenuItem, MenuCategory } from "./PluginDefinition";

interface FilePreviewMenuContextType {
    registeredItems: Record<string, PluginMenuItem[]>;
    registerMenu: (id: string, items: PluginMenuItem[]) => void;
    unregisterMenu: (id: string) => void;
}

const FilePreviewMenuContext = createContext<FilePreviewMenuContextType | null>(null);

export const FilePreviewMenuProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
    const [registeredItems, setRegisteredItems] = useState<Record<string, PluginMenuItem[]>>({});

    const registerMenu = useCallback((id: string, items: PluginMenuItem[]) => {
        setRegisteredItems((prev) => {
            const existing = prev[id];
            if (existing && existing.length === items.length) {
                const isEqual = existing.every(
                    (item, idx) =>
                        item.id === items[idx].id &&
                        item.label === items[idx].label &&
                        item.disabled === items[idx].disabled &&
                        item.category === items[idx].category
                );
                if (isEqual) return prev;
            }
            return {
                ...prev,
                [id]: items,
            };
        });
    }, []);

    const unregisterMenu = useCallback((id: string) => {
        setRegisteredItems((prev) => {
            if (!(id in prev)) return prev;
            const next = { ...prev };
            delete next[id];
            return next;
        });
    }, []);

    const contextValue = useMemo(
        () => ({ registeredItems, registerMenu, unregisterMenu }),
        [registeredItems, registerMenu, unregisterMenu]
    );

    return (
        <FilePreviewMenuContext.Provider value={contextValue}>
            {children}
        </FilePreviewMenuContext.Provider>
    );
};

export const useFilePreviewMenuContext = () => {
    return useContext(FilePreviewMenuContext);
};

export function useRegisterMenu(
    categoryOrItems: MenuCategory | PluginMenuItem[],
    itemsOrUndefined?: PluginMenuItem[]
) {
    const context = useContext(FilePreviewMenuContext);
    const registrationId = useId();

    const category = typeof categoryOrItems === "string" ? categoryOrItems : undefined;
    const items = Array.isArray(categoryOrItems) ? categoryOrItems : (itemsOrUndefined || []);

    const registerMenu = context?.registerMenu;
    const unregisterMenu = context?.unregisterMenu;

    const itemsRef = useRef(items);
    itemsRef.current = items;

    // 1. Sync registration & updates with context (backed by registerMenu's deep-equal check)
    useEffect(() => {
        if (!registerMenu || !itemsRef.current.length) return;

        const currentItems = itemsRef.current;
        const normalizedItems = category
            ? currentItems.map((item) => ({ ...item, category }))
            : currentItems;

        registerMenu(registrationId, normalizedItems);
    });

    // 2. Unregister items ONLY when component unmounts
    useEffect(() => {
        if (!unregisterMenu) return;
        return () => {
            unregisterMenu(registrationId);
        };
    }, [unregisterMenu, registrationId]);
}
