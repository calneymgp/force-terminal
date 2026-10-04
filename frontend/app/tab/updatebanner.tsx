// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { WaveEnv, WaveEnvSubset, useWaveEnv } from "@/app/waveenv/waveenv";
import { useAtomValue } from "jotai";
import { memo } from "react";

type UpdateBannerEnv = WaveEnvSubset<{
    electron: {
        installAppUpdate: WaveEnv["electron"]["installAppUpdate"];
    };
    atoms: {
        updaterStatusAtom: WaveEnv["atoms"]["updaterStatusAtom"];
    };
}>;

function getUpdateStatusMessage(state: UpdaterState): string {
    switch (state.status) {
        case "pending": return "Check updates";
        case "checking": return "Checking updates";
        case "up-to-date": return "Up to date";
        case "available": return `Update ${state.version ?? "available"}`;
        case "downloading": return `Downloading ${Math.round(state.percent ?? 0)}%`;
        case "ready": return state.blockedReasons?.length ? "Update delayed" : "Restart to update";
        case "installing": return "Installing update";
        case "error": return "Retry update";
        case "dev-disabled": return "Updates unavailable in development";
    }
}

const UpdateStatusBannerComponent = () => {
    const env = useWaveEnv<UpdateBannerEnv>();
    const appUpdateStatus = useAtomValue(env.atoms.updaterStatusAtom);
    const updateStatusMessage = getUpdateStatusMessage(appUpdateStatus);
    const disabled = ["dev-disabled", "downloading", "installing"].includes(appUpdateStatus.status);
    const reason = appUpdateStatus.blockedReasons?.join("; ") || appUpdateStatus.error;

    return (
        <button
            type="button"
            title={reason ? `${updateStatusMessage}: ${reason}` : updateStatusMessage}
            aria-label={reason ? `${updateStatusMessage}: ${reason}` : updateStatusMessage}
            disabled={disabled}
            onClick={() => env.electron.installAppUpdate()}
            className="flex items-center gap-1 px-2 mb-1 h-[22px] text-xs font-medium text-secondary rounded-sm hover:bg-hoverbg disabled:cursor-default disabled:opacity-70"
            style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
        >
            <i className="fa fa-download" aria-hidden="true" />
            {updateStatusMessage}
        </button>
    );
};
UpdateStatusBannerComponent.displayName = "UpdateStatusBannerComponent";

export const UpdateStatusBanner = memo(UpdateStatusBannerComponent);
