// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

import { ForceService } from "@/app/store/services";
import { waveEventSubscribeSingle } from "@/app/store/wps";
import { addWSReconnectHandler, removeWSReconnectHandler } from "@/app/store/ws";
import { useCallback, useEffect, useRef, useState } from "react";

export type CatalogProject = ForceProject;
export type CatalogProfile = ForceAgentProfile;
export type Catalog = ForceCatalog;

const pendingForms = new Map<string, string>();

export function setForceCatalogUpdateReason(id: string, reason: string | null): void {
    if (reason) pendingForms.set(id, reason);
    else pendingForms.delete(id);
}

export function getForceCatalogUpdateReasons(): string[] {
    return [...pendingForms.values()];
}

export function errorMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    return String(error);
}

export function useForceCatalog(initialCatalog?: Catalog) {
    const [catalog, setCatalog] = useState<Catalog>(initialCatalog ?? { projects: [], profiles: [] });
    const [loading, setLoading] = useState(!initialCatalog);
    const [error, setError] = useState<string | null>(null);
    const requestNumber = useRef(0);
    const mounted = useRef(true);
    const refresh = useCallback(async () => {
        const current = ++requestNumber.current;
        try {
            const next = await ForceService.GetCatalog();
            if (!mounted.current || current !== requestNumber.current) return;
            setCatalog(next);
            setError(null);
        } catch (err) {
            if (mounted.current && current === requestNumber.current) setError(errorMessage(err));
        } finally {
            if (mounted.current && current === requestNumber.current) setLoading(false);
        }
    }, []);

    useEffect(() => {
        mounted.current = true;
        if (initialCatalog) return;
        void refresh();
        const unsubscribe = waveEventSubscribeSingle({
            eventType: "waveobj:update",
            handler: (event) => {
                if (event.data?.otype === "forceproject" || event.data?.otype === "forceagentprofile") {
                    void refresh();
                }
            },
        });
        const onFocus = () => void refresh();
        const onVisible = () => { if (document.visibilityState === "visible") void refresh(); };
        addWSReconnectHandler(onFocus);
        window.addEventListener("focus", onFocus);
        document.addEventListener("visibilitychange", onVisible);
        return () => {
            mounted.current = false;
            requestNumber.current++;
            unsubscribe();
            removeWSReconnectHandler(onFocus);
            window.removeEventListener("focus", onFocus);
            document.removeEventListener("visibilitychange", onVisible);
        };
    }, [initialCatalog, refresh]);

    return { catalog, loading, error, refresh };
}
