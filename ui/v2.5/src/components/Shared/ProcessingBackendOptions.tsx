// Values are shared by the existing conversion, restoration and overlap APIs.
export function ProcessingBackendOptions() {
  return (
    <>
      <option value="auto">Automatic — prefer remote worker</option>
      <option value="local">This server only</option>
      <option value="remote">Remote worker only</option>
    </>
  );
}
