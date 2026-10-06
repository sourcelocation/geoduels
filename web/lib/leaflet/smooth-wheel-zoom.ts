/*
 * Adapted from Leaflet.SmoothWheelZoom at commit
 * a68e52f46315f93ed560b8314cb7235df947ea32:
 * https://github.com/mutsuyuki/Leaflet.SmoothWheelZoom
 * Uses an explicit Leaflet import and cancels pending work on disable/unmount.
 * Zooming at the exact map center is supported as well as around the cursor.
 *
 * MIT License
 * Copyright (c) 2018 mutsuyuki
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */
import L from 'leaflet';

// The upstream plugin uses Leaflet's internal movement methods for continuous
// fractional zoom, without starting a separate animation for each wheel event.
type SmoothZoomMap = L.Map & {
  _stop: () => void;
  _panAnim?: { stop: () => void };
  _moveStart: (zoomChanged: boolean, noMoveStart: boolean) => L.Map;
  _move: (center: L.LatLng, zoom: number) => L.Map;
  _moveEnd: (zoomChanged: boolean) => L.Map;
};

type WheelGesture = {
  centerPoint: L.Point;
  mousePosition: L.Point;
  mouseLatLng: L.LatLng;
  goalZoom: number;
  previousCenter: L.LatLng;
  previousZoom: number;
  moved: boolean;
};

export class SmoothWheelZoom extends L.Handler {
  private readonly map: SmoothZoomMap;
  private gesture: WheelGesture | null = null;
  private zoomFrame?: number;
  private wheelEndTimeout?: number;

  constructor(map: L.Map, private readonly sensitivity = 1) {
    super(map);
    this.map = map as SmoothZoomMap;
  }

  addHooks() {
    L.DomEvent.on(this.map.getContainer(), 'wheel', this.onWheelScroll, this);
    this.map.on('unload', this.disable, this);
  }

  removeHooks() {
    L.DomEvent.off(this.map.getContainer(), 'wheel', this.onWheelScroll, this);
    this.map.off('unload', this.disable, this);
    // Map.remove() may already have removed the panes. Cancel without firing
    // movement events against layers that are being torn down.
    this.cancelGesture();
  }

  private cancelGesture() {
    window.clearTimeout(this.wheelEndTimeout);
    if (this.zoomFrame !== undefined) cancelAnimationFrame(this.zoomFrame);
    this.wheelEndTimeout = undefined;
    this.zoomFrame = undefined;
    this.gesture = null;
  }

  private onWheelScroll(wheelEvent: Event) {
    const event = wheelEvent as WheelEvent;
    const map = this.map;
    if (!this.gesture) {
      map._stop();
      map._panAnim?.stop();
      const mousePosition = map.mouseEventToContainerPoint(event);
      this.gesture = {
        centerPoint: map.getSize().divideBy(2),
        mousePosition,
        mouseLatLng: map.containerPointToLatLng(mousePosition),
        goalZoom: map.getZoom(),
        previousCenter: map.getCenter(),
        previousZoom: map.getZoom(),
        moved: false,
      };
      this.zoomFrame = requestAnimationFrame(this.updateWheelZoom);
    }

    const gesture = this.gesture;
    gesture.goalZoom = Math.max(map.getMinZoom(), Math.min(map.getMaxZoom(),
      gesture.goalZoom + L.DomEvent.getWheelDelta(event) * 0.003 * this.sensitivity));
    gesture.mousePosition = map.mouseEventToContainerPoint(event);
    gesture.mouseLatLng = map.containerPointToLatLng(gesture.mousePosition);
    window.clearTimeout(this.wheelEndTimeout);
    this.wheelEndTimeout = window.setTimeout(this.endWheelZoom, 200);
    L.DomEvent.preventDefault(event);
    L.DomEvent.stopPropagation(event);
  }

  private endWheelZoom = () => {
    const moved = this.gesture?.moved;
    this.cancelGesture();
    if (moved) this.map._moveEnd(true);
  };

  private updateWheelZoom = () => {
    const map = this.map;
    const gesture = this.gesture;
    if (!gesture) return;
    if (!map.getCenter().equals(gesture.previousCenter) || map.getZoom() !== gesture.previousZoom) {
      this.endWheelZoom();
      return;
    }

    const zoom = Math.floor((map.getZoom() + (gesture.goalZoom - map.getZoom()) * 0.3) * 100) / 100;
    const delta = gesture.mousePosition.subtract(gesture.centerPoint);
    const center = map.unproject(map.project(gesture.mouseLatLng, zoom).subtract(delta), zoom);
    if (!gesture.moved) {
      map._moveStart(true, false);
      gesture.moved = true;
    }
    map._move(center, zoom);
    if (this.gesture !== gesture) return;
    gesture.previousCenter = map.getCenter();
    gesture.previousZoom = map.getZoom();
    this.zoomFrame = requestAnimationFrame(this.updateWheelZoom);
  };
}
