// copies the methods of a helper class onto Agent, so one class can live in several files
export function mixin(Target, Source) {
  for (const name of Object.getOwnPropertyNames(Source.prototype)) {
    if (name !== 'constructor') Target.prototype[name] = Source.prototype[name];
  }
}
