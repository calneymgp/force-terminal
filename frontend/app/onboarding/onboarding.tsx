// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import Logo from "@/app/asset/logo-detailed.svg";
import { Button } from "@/app/element/button";
import { FlexiModal } from "@/app/modals/modal";
import { CurrentOnboardingVersion, OnboardingGradientBg } from "@/app/onboarding/onboarding-common";
import { ClientModel } from "@/app/store/client-model";
import { useSettingsKeyAtom } from "@/app/store/global";
import { disableGlobalKeybindings, enableGlobalKeybindings, globalRefocus } from "@/app/store/keymodel";
import { modalsModel } from "@/app/store/modalmodel";
import * as WOS from "@/app/store/wos";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import * as services from "@/store/services";
import { useAtom, useAtomValue } from "jotai";
import { OverlayScrollbarsComponent } from "overlayscrollbars-react";
import { useEffect, useRef, useState } from "react";
import { debounce } from "throttle-debounce";

const InitPage = ({
    isCompact,
    telemetryUpdateFn,
    onContinue,
    isSaving,
    saveError,
}: {
    isCompact: boolean;
    telemetryUpdateFn: (value: boolean) => Promise<void>;
    onContinue: () => Promise<void>;
    isSaving: boolean;
    saveError: string | null;
}) => {
    const telemetrySetting = useSettingsKeyAtom("telemetry:enabled");
    const [telemetryEnabled, setTelemetryEnabled] = useState<boolean>(!!telemetrySetting);
    const [isSavingTelemetry, setIsSavingTelemetry] = useState(false);
    const [telemetryError, setTelemetryError] = useState<string | null>(null);
    const telemetryUpdateInFlight = useRef(false);

    const handleStarClick = async () => {
        RpcApi.RecordTEventCommand(
            TabRpcClient,
            {
                event: "onboarding:githubstar",
                props: { "onboarding:githubstar": "star", "onboarding:page": "init" },
            },
            { noresponse: true }
        );
        const clientId = ClientModel.getInstance().clientId;
        await RpcApi.SetMetaCommand(TabRpcClient, {
            oref: WOS.makeORef("client", clientId),
            meta: { "onboarding:githubstar": true },
        });
    };

    const setTelemetry = async (value: boolean) => {
        if (telemetryUpdateInFlight.current || isSaving) {
            return;
        }
        telemetryUpdateInFlight.current = true;
        setIsSavingTelemetry(true);
        setTelemetryError(null);
        try {
            await telemetryUpdateFn(value);
            setTelemetryEnabled(value);
            setTelemetryError(null);
        } catch {
            setTelemetryError("Couldn't save your telemetry setting. Change your choice to retry it.");
        } finally {
            telemetryUpdateInFlight.current = false;
            setIsSavingTelemetry(false);
        }
    };

    const handleContinue = async () => {
        if (isSaving || isSavingTelemetry || telemetryUpdateInFlight.current || telemetryError) {
            return;
        }
        await onContinue();
    };

    const label = telemetryEnabled ? "Enabled" : "Disabled";

    return (
        <div className="flex flex-col h-full">
            <header
                className={`flex flex-col gap-2 border-b-0 p-0 ${isCompact ? "mt-1 mb-4" : "mb-9"} w-full unselectable flex-shrink-0`}
            >
                <div className={`${isCompact ? "" : "mb-2.5"} flex justify-center`}>
                    <Logo />
                </div>
                <div className="text-center text-[25px] font-normal text-foreground">Welcome to Force Terminal</div>
            </header>
            <OverlayScrollbarsComponent
                className="flex-1 overflow-y-auto min-h-0"
                options={{ scrollbars: { autoHide: "never" } }}
            >
                <div className="flex flex-col items-start gap-8 w-full mb-5 unselectable">
                    <div className="flex w-full items-center gap-[18px]">
                        <div>
                            <a
                                target="_blank"
                                href="https://github.com/calneymgp/force-terminal?ref=install"
                                rel="noopener"
                                className="text-accent"
                                onClick={handleStarClick}
                            >
                                <i className="text-[32px] text-white/50 fa-brands fa-github"></i>
                            </a>
                        </div>
                        <div className="flex flex-col items-start gap-1 flex-1">
                            <div className="text-foreground text-base leading-[18px]">Support Force Terminal on GitHub</div>
                            <div className="text-secondary leading-5">
                                We're <i>open source</i>, <i>open-model</i>, and committed to providing a free terminal
                                for individual users. Please show your support by giving us a star on{" "}
                                <a
                                    target="_blank"
                                    href="https://github.com/calneymgp/force-terminal?ref=install"
                                    rel="noopener"
                                    className="text-accent"
                                    onClick={handleStarClick}
                                >
                                    GitHub&nbsp;(calneymgp/force-terminal)
                                </a>
                            </div>
                        </div>
                    </div>
                    <div className="flex w-full items-center gap-[18px]">
                        <div>
                            <a
                                target="_blank"
                                href="https://discord.gg/XfvZ334gwU"
                                rel="noopener"
                                className="text-accent"
                            >
                                <i className="text-[25px] text-white/50 fa-solid fa-people-group"></i>
                            </a>
                        </div>
                        <div className="flex flex-col items-start gap-1 flex-1">
                            <div className="text-foreground text-base leading-[18px]">Original Wave Community</div>
                            <div className="text-secondary leading-5">
                                Get help, submit feature requests, report bugs, or just chat with fellow terminal
                                enthusiasts.
                                <br />
                                <a
                                    target="_blank"
                                    href="https://discord.gg/XfvZ334gwU"
                                    rel="noopener"
                                    className="text-accent"
                                >
                                    Join the original Wave&nbsp;Discord&nbsp;community
                                </a>
                            </div>
                        </div>
                    </div>
                    <div className="flex w-full items-center gap-[18px]">
                        <div>
                            <i className="text-[32px] text-white/50 fa-solid fa-chart-line"></i>
                        </div>
                        <div className="flex flex-col items-start gap-1 flex-1">
                            <div className="text-secondary leading-5">
                                Anonymous usage data helps us improve features you use.
                                <br />
                                <a
                                    className="text-secondary! hover:underline!"
                                    target="_blank"
                                    href="https://waveterm.dev/privacy"
                                    rel="noopener"
                                >
                                    Original Wave Privacy Policy
                                </a>
                            </div>
                            <label className="flex items-center gap-2 cursor-pointer text-secondary">
                                <input
                                    type="checkbox"
                                    checked={telemetryEnabled}
                                    onChange={(e) => setTelemetry(e.target.checked)}
                                    disabled={isSaving || isSavingTelemetry}
                                    className="cursor-pointer accent-gray-500"
                                />
                                <span>{label}</span>
                            </label>
                        </div>
                    </div>
                </div>
            </OverlayScrollbarsComponent>
            {(saveError || telemetryError) && (
                <div
                    role="alert"
                    className="flex items-center gap-2 mt-3 p-3 border rounded-lg bg-red-500/10 border-red-500/20 text-red-400"
                >
                    <i className="fa-sharp fa-solid fa-circle-exclamation" />
                    <span>{saveError || telemetryError}</span>
                </div>
            )}
            <footer className={`unselectable flex-shrink-0 ${isCompact ? "mt-2" : "mt-5"}`}>
                <div className="flex flex-row items-center justify-center [&>button]:!px-5 [&>button]:!py-2 [&>button]:text-sm [&>button:not(:first-child)]:ml-2.5">
                    <Button
                        className="font-[600]"
                        onClick={handleContinue}
                        disabled={isSaving || isSavingTelemetry || !!telemetryError}
                    >
                        Continue
                    </Button>
                </div>
            </footer>
        </div>
    );
};

const NewInstallOnboardingModal = () => {
    const modalRef = useRef<HTMLDivElement | null>(null);
    const [, setNewInstallOnboardingOpen] = useAtom(modalsModel.newInstallOnboardingOpen);
    const clientData = useAtomValue(ClientModel.getInstance().clientAtom);
    const [isCompact, setIsCompact] = useState<boolean>(window.innerHeight < 800);
    const [isSaving, setIsSaving] = useState(false);
    const [saveError, setSaveError] = useState<string | null>(null);
    const continueInFlight = useRef(false);

    const handleContinue = async () => {
        if (continueInFlight.current) {
            return;
        }
        continueInFlight.current = true;
        setIsSaving(true);
        setSaveError(null);
        try {
            if (!clientData?.tosagreed) {
                await services.ClientService.AgreeTos();
            }
            const clientId = ClientModel.getInstance().clientId;
            await RpcApi.SetMetaCommand(TabRpcClient, {
                oref: WOS.makeORef("client", clientId),
                meta: { "onboarding:lastversion": CurrentOnboardingVersion },
            });
            setNewInstallOnboardingOpen(false);
            setTimeout(() => {
                globalRefocus();
            }, 10);
        } catch {
            continueInFlight.current = false;
            setIsSaving(false);
            setSaveError("Couldn't save your agreement. Please try again.");
        }
    };

    const updateModalHeight = () => {
        const windowHeight = window.innerHeight;
        setIsCompact(windowHeight < 800);
        if (modalRef.current) {
            const modalHeight = modalRef.current.offsetHeight;
            const maxHeight = windowHeight * 0.9;
            if (maxHeight < modalHeight) {
                modalRef.current.style.height = `${maxHeight}px`;
            } else {
                modalRef.current.style.height = "auto";
            }
        }
    };

    useEffect(() => {
        updateModalHeight();
        const debouncedUpdateModalHeight = debounce(150, updateModalHeight);
        window.addEventListener("resize", debouncedUpdateModalHeight);
        return () => {
            window.removeEventListener("resize", debouncedUpdateModalHeight);
        };
    }, []);

    useEffect(() => {
        disableGlobalKeybindings();
        return () => {
            enableGlobalKeybindings();
        };
    }, []);

    const paddingClass = isCompact ? "!py-3 !px-[30px]" : "!p-[30px]";

    return (
        <FlexiModal className={`w-[560px] rounded-[10px] ${paddingClass} relative overflow-hidden`} ref={modalRef}>
            <OnboardingGradientBg />
            <div className="flex flex-col w-full h-full relative z-10">
                <InitPage
                    isCompact={isCompact}
                    telemetryUpdateFn={(value) => services.ClientService.TelemetryUpdate(value)}
                    onContinue={handleContinue}
                    isSaving={isSaving}
                    saveError={saveError}
                />
            </div>
        </FlexiModal>
    );
};

NewInstallOnboardingModal.displayName = "NewInstallOnboardingModal";

export { InitPage, NewInstallOnboardingModal };
