import { onBeforeUnmount, watch, type Ref } from "vue";
import { onBeforeRouteLeave } from "vue-router";

export function useUnsavedChanges(
  isDirty: Ref<boolean>,
  confirmLeave: (message: string) => Promise<boolean>,
  message = "You have unsaved changes. Leave without saving?"
) {
  function onBeforeUnload(e: BeforeUnloadEvent) {
    if (!isDirty.value) return;
    e.preventDefault();
    // Browsers ignore custom text here, but older engines need the assignment to trigger the prompt.
    e.returnValue = message;
  }

  // Attached only while there is something to lose, so a clean editor never makes the browser hesitate on reload.
  watch(
    isDirty,
    (dirty) => {
      if (dirty) window.addEventListener("beforeunload", onBeforeUnload);
      else window.removeEventListener("beforeunload", onBeforeUnload);
    },
    { immediate: true }
  );

  onBeforeUnmount(() => window.removeEventListener("beforeunload", onBeforeUnload));

  onBeforeRouteLeave(async () => {
    if (!isDirty.value) return true;
    return await confirmLeave(message);
  });
}
