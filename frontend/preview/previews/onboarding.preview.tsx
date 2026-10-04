// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { InitPage } from "@/app/onboarding/onboarding";
import { OnboardingGradientBg } from "@/app/onboarding/onboarding-common";

function OnboardingModalWrapper({ children }: { children: React.ReactNode }) {
    return (
        <div className="w-[560px] rounded-[10px] p-[30px] relative overflow-hidden bg-panel">
            <OnboardingGradientBg />
            <div className="relative z-10 flex flex-col w-full h-full">{children}</div>
        </div>
    );
}

export function OnboardingPreview() {
    return (
        <div className="w-full max-w-[1300px] py-10 px-4 flex flex-col gap-8">
            <div className="text-sm font-mono text-muted">Welcome to Force Terminal</div>
            <OnboardingModalWrapper>
                <InitPage
                    isCompact={false}
                    telemetryUpdateFn={async () => {}}
                    onContinue={async () => {}}
                    isSaving={false}
                    saveError={null}
                />
            </OnboardingModalWrapper>
        </div>
    );
}
