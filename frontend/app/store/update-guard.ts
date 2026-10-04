type LoadedModel = { viewModel?: any };

export async function prepareLoadedModels(models: LoadedModel[], read: (atom: any) => any): Promise<{ reasons: string[]; verifiedIdleBlocks: string[] }> {
    const reasons: string[] = [];
    const verifiedIdleBlocks: string[] = [];
    for (const { viewModel: model } of models) {
        if (!model) continue;
        if (model.viewType === "preview" && read(model.newFileContent) != null) {
            try {
                await model.handleFileSave();
            } catch (error) {
                reasons.push(`Preview save failed: ${String(error)}`);
            }
            if (read(model.newFileContent) != null) reasons.push("Preview file still has unsaved changes");
        }
        if (model.viewType === "waveconfig" && read(model.hasEditedAtom)) {
            try {
                await model.saveFile();
            } catch (error) {
                reasons.push(`Configuration save failed: ${String(error)}`);
            }
            if (read(model.hasEditedAtom)) reasons.push("Configuration still has unsaved changes");
        }
        if (model.viewType === "term" && model.termRef?.current?.shellIntegrationStatusAtom &&
            read(model.termRef.current.shellIntegrationStatusAtom) === "ready") {
            verifiedIdleBlocks.push(model.blockId);
        }
    }
    return { reasons, verifiedIdleBlocks };
}
