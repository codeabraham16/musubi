(()=>{var bi={LEFT:0,MIDDLE:1,RIGHT:2,ROTATE:0,DOLLY:1,PAN:2};var Ud=0,rh=1,Nd=2;var lu=1,Od=2,Cn=3,jn=0,Fe=1,Rn=2,vn=0,is=1,as=2,oh=3,ah=4,Fd=5,fi=100,Bd=101,zd=102,kd=103,Hd=104,Vd=200,Gd=201,Wd=202,Xd=203,Va=204,Ga=205,qd=206,Yd=207,Zd=208,jd=209,Kd=210,Jd=211,Qd=212,$d=213,tf=214,Wa=0,Xa=1,qa=2,ls=3,Ya=4,Za=5,ja=6,Ka=7,cu=0,ef=1,nf=2,Zn=0,sf=1,rf=2,of=3,af=4,lf=5,cf=6,hf=7;var hu=300,cs=301,hs=302,Ja=303,Qa=304,wo=306,$a=1e3,gi=1001,tl=1002,be=1003,uf=1004;var yr=1005;var Xe=1006,da=1007;var xi=1008;var In=1009,uu=1010,du=1011,Qs=1012,rc=1013,vi=1014,xn=1015,ze=1016,oc=1017,ac=1018,us=1020,fu=35902,pu=1021,mu=1022,hn=1023,gu=1024,xu=1025,ss=1026,ds=1027,lc=1028,cc=1029,vu=1030,hc=1031;var uc=1033,Gr=33776,Wr=33777,Xr=33778,qr=33779,el=35840,nl=35841,il=35842,sl=35843,rl=36196,ol=37492,al=37496,ll=37808,cl=37809,hl=37810,ul=37811,dl=37812,fl=37813,pl=37814,ml=37815,gl=37816,xl=37817,vl=37818,_l=37819,yl=37820,Ml=37821,Yr=36492,Sl=36494,bl=36495,_u=36283,wl=36284,Al=36285,El=36286;var jr=2300,Tl=2301,fa=2302,lh=2400,ch=2401,hh=2402;var df=3200,ff=3201;var yu=0,pf=1,qn="",mn="srgb",Kn="srgb-linear",dc="display-p3",Ao="display-p3-linear",Kr="linear",ee="srgb",Jr="rec709",Qr="p3";var ki=7680;var uh=519,mf=512,gf=513,xf=514,Mu=515,vf=516,_f=517,yf=518,Mf=519,dh=35044;var fh="300 es",Pn=2e3,$r=2001,Ln=class{addEventListener(t,e){this._listeners===void 0&&(this._listeners={});let i=this._listeners;i[t]===void 0&&(i[t]=[]),i[t].indexOf(e)===-1&&i[t].push(e)}hasEventListener(t,e){if(this._listeners===void 0)return!1;let i=this._listeners;return i[t]!==void 0&&i[t].indexOf(e)!==-1}removeEventListener(t,e){if(this._listeners===void 0)return;let s=this._listeners[t];if(s!==void 0){let r=s.indexOf(e);r!==-1&&s.splice(r,1)}}dispatchEvent(t){if(this._listeners===void 0)return;let i=this._listeners[t.type];if(i!==void 0){t.target=this;let s=i.slice(0);for(let r=0,o=s.length;r<o;r++)s[r].call(this,t);t.target=null}}},Ee=["00","01","02","03","04","05","06","07","08","09","0a","0b","0c","0d","0e","0f","10","11","12","13","14","15","16","17","18","19","1a","1b","1c","1d","1e","1f","20","21","22","23","24","25","26","27","28","29","2a","2b","2c","2d","2e","2f","30","31","32","33","34","35","36","37","38","39","3a","3b","3c","3d","3e","3f","40","41","42","43","44","45","46","47","48","49","4a","4b","4c","4d","4e","4f","50","51","52","53","54","55","56","57","58","59","5a","5b","5c","5d","5e","5f","60","61","62","63","64","65","66","67","68","69","6a","6b","6c","6d","6e","6f","70","71","72","73","74","75","76","77","78","79","7a","7b","7c","7d","7e","7f","80","81","82","83","84","85","86","87","88","89","8a","8b","8c","8d","8e","8f","90","91","92","93","94","95","96","97","98","99","9a","9b","9c","9d","9e","9f","a0","a1","a2","a3","a4","a5","a6","a7","a8","a9","aa","ab","ac","ad","ae","af","b0","b1","b2","b3","b4","b5","b6","b7","b8","b9","ba","bb","bc","bd","be","bf","c0","c1","c2","c3","c4","c5","c6","c7","c8","c9","ca","cb","cc","cd","ce","cf","d0","d1","d2","d3","d4","d5","d6","d7","d8","d9","da","db","dc","dd","de","df","e0","e1","e2","e3","e4","e5","e6","e7","e8","e9","ea","eb","ec","ed","ee","ef","f0","f1","f2","f3","f4","f5","f6","f7","f8","f9","fa","fb","fc","fd","fe","ff"],ph=1234567,js=Math.PI/180,$s=180/Math.PI;function xs(){let n=Math.random()*4294967295|0,t=Math.random()*4294967295|0,e=Math.random()*4294967295|0,i=Math.random()*4294967295|0;return(Ee[n&255]+Ee[n>>8&255]+Ee[n>>16&255]+Ee[n>>24&255]+"-"+Ee[t&255]+Ee[t>>8&255]+"-"+Ee[t>>16&15|64]+Ee[t>>24&255]+"-"+Ee[e&63|128]+Ee[e>>8&255]+"-"+Ee[e>>16&255]+Ee[e>>24&255]+Ee[i&255]+Ee[i>>8&255]+Ee[i>>16&255]+Ee[i>>24&255]).toLowerCase()}function Le(n,t,e){return Math.max(t,Math.min(e,n))}function fc(n,t){return(n%t+t)%t}function Sf(n,t,e,i,s){return i+(n-t)*(s-i)/(e-t)}function bf(n,t,e){return n!==t?(e-n)/(t-n):0}function Ks(n,t,e){return(1-e)*n+e*t}function wf(n,t,e,i){return Ks(n,t,1-Math.exp(-e*i))}function Af(n,t=1){return t-Math.abs(fc(n,t*2)-t)}function Ef(n,t,e){return n<=t?0:n>=e?1:(n=(n-t)/(e-t),n*n*(3-2*n))}function Tf(n,t,e){return n<=t?0:n>=e?1:(n=(n-t)/(e-t),n*n*n*(n*(n*6-15)+10))}function Cf(n,t){return n+Math.floor(Math.random()*(t-n+1))}function Rf(n,t){return n+Math.random()*(t-n)}function Pf(n){return n*(.5-Math.random())}function If(n){n!==void 0&&(ph=n);let t=ph+=1831565813;return t=Math.imul(t^t>>>15,t|1),t^=t+Math.imul(t^t>>>7,t|61),((t^t>>>14)>>>0)/4294967296}function Lf(n){return n*js}function Df(n){return n*$s}function Uf(n){return(n&n-1)===0&&n!==0}function Nf(n){return Math.pow(2,Math.ceil(Math.log(n)/Math.LN2))}function Of(n){return Math.pow(2,Math.floor(Math.log(n)/Math.LN2))}function Ff(n,t,e,i,s){let r=Math.cos,o=Math.sin,a=r(e/2),l=o(e/2),h=r((t+i)/2),f=o((t+i)/2),d=r((t-i)/2),c=o((t-i)/2),u=r((i-t)/2),g=o((i-t)/2);switch(s){case"XYX":n.set(a*f,l*d,l*c,a*h);break;case"YZY":n.set(l*c,a*f,l*d,a*h);break;case"ZXZ":n.set(l*d,l*c,a*f,a*h);break;case"XZX":n.set(a*f,l*g,l*u,a*h);break;case"YXY":n.set(l*u,a*f,l*g,a*h);break;case"ZYZ":n.set(l*g,l*u,a*f,a*h);break;default:console.warn("THREE.MathUtils: .setQuaternionFromProperEuler() encountered an unknown order: "+s)}}function es(n,t){switch(t.constructor){case Float32Array:return n;case Uint32Array:return n/4294967295;case Uint16Array:return n/65535;case Uint8Array:return n/255;case Int32Array:return Math.max(n/2147483647,-1);case Int16Array:return Math.max(n/32767,-1);case Int8Array:return Math.max(n/127,-1);default:throw new Error("Invalid component type.")}}function Pe(n,t){switch(t.constructor){case Float32Array:return n;case Uint32Array:return Math.round(n*4294967295);case Uint16Array:return Math.round(n*65535);case Uint8Array:return Math.round(n*255);case Int32Array:return Math.round(n*2147483647);case Int16Array:return Math.round(n*32767);case Int8Array:return Math.round(n*127);default:throw new Error("Invalid component type.")}}var pc={DEG2RAD:js,RAD2DEG:$s,generateUUID:xs,clamp:Le,euclideanModulo:fc,mapLinear:Sf,inverseLerp:bf,lerp:Ks,damp:wf,pingpong:Af,smoothstep:Ef,smootherstep:Tf,randInt:Cf,randFloat:Rf,randFloatSpread:Pf,seededRandom:If,degToRad:Lf,radToDeg:Df,isPowerOfTwo:Uf,ceilPowerOfTwo:Nf,floorPowerOfTwo:Of,setQuaternionFromProperEuler:Ff,normalize:Pe,denormalize:es},yt=class n{constructor(t=0,e=0){n.prototype.isVector2=!0,this.x=t,this.y=e}get width(){return this.x}set width(t){this.x=t}get height(){return this.y}set height(t){this.y=t}set(t,e){return this.x=t,this.y=e,this}setScalar(t){return this.x=t,this.y=t,this}setX(t){return this.x=t,this}setY(t){return this.y=t,this}setComponent(t,e){switch(t){case 0:this.x=e;break;case 1:this.y=e;break;default:throw new Error("index is out of range: "+t)}return this}getComponent(t){switch(t){case 0:return this.x;case 1:return this.y;default:throw new Error("index is out of range: "+t)}}clone(){return new this.constructor(this.x,this.y)}copy(t){return this.x=t.x,this.y=t.y,this}add(t){return this.x+=t.x,this.y+=t.y,this}addScalar(t){return this.x+=t,this.y+=t,this}addVectors(t,e){return this.x=t.x+e.x,this.y=t.y+e.y,this}addScaledVector(t,e){return this.x+=t.x*e,this.y+=t.y*e,this}sub(t){return this.x-=t.x,this.y-=t.y,this}subScalar(t){return this.x-=t,this.y-=t,this}subVectors(t,e){return this.x=t.x-e.x,this.y=t.y-e.y,this}multiply(t){return this.x*=t.x,this.y*=t.y,this}multiplyScalar(t){return this.x*=t,this.y*=t,this}divide(t){return this.x/=t.x,this.y/=t.y,this}divideScalar(t){return this.multiplyScalar(1/t)}applyMatrix3(t){let e=this.x,i=this.y,s=t.elements;return this.x=s[0]*e+s[3]*i+s[6],this.y=s[1]*e+s[4]*i+s[7],this}min(t){return this.x=Math.min(this.x,t.x),this.y=Math.min(this.y,t.y),this}max(t){return this.x=Math.max(this.x,t.x),this.y=Math.max(this.y,t.y),this}clamp(t,e){return this.x=Math.max(t.x,Math.min(e.x,this.x)),this.y=Math.max(t.y,Math.min(e.y,this.y)),this}clampScalar(t,e){return this.x=Math.max(t,Math.min(e,this.x)),this.y=Math.max(t,Math.min(e,this.y)),this}clampLength(t,e){let i=this.length();return this.divideScalar(i||1).multiplyScalar(Math.max(t,Math.min(e,i)))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this}negate(){return this.x=-this.x,this.y=-this.y,this}dot(t){return this.x*t.x+this.y*t.y}cross(t){return this.x*t.y-this.y*t.x}lengthSq(){return this.x*this.x+this.y*this.y}length(){return Math.sqrt(this.x*this.x+this.y*this.y)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)}normalize(){return this.divideScalar(this.length()||1)}angle(){return Math.atan2(-this.y,-this.x)+Math.PI}angleTo(t){let e=Math.sqrt(this.lengthSq()*t.lengthSq());if(e===0)return Math.PI/2;let i=this.dot(t)/e;return Math.acos(Le(i,-1,1))}distanceTo(t){return Math.sqrt(this.distanceToSquared(t))}distanceToSquared(t){let e=this.x-t.x,i=this.y-t.y;return e*e+i*i}manhattanDistanceTo(t){return Math.abs(this.x-t.x)+Math.abs(this.y-t.y)}setLength(t){return this.normalize().multiplyScalar(t)}lerp(t,e){return this.x+=(t.x-this.x)*e,this.y+=(t.y-this.y)*e,this}lerpVectors(t,e,i){return this.x=t.x+(e.x-t.x)*i,this.y=t.y+(e.y-t.y)*i,this}equals(t){return t.x===this.x&&t.y===this.y}fromArray(t,e=0){return this.x=t[e],this.y=t[e+1],this}toArray(t=[],e=0){return t[e]=this.x,t[e+1]=this.y,t}fromBufferAttribute(t,e){return this.x=t.getX(e),this.y=t.getY(e),this}rotateAround(t,e){let i=Math.cos(e),s=Math.sin(e),r=this.x-t.x,o=this.y-t.y;return this.x=r*i-o*s+t.x,this.y=r*s+o*i+t.y,this}random(){return this.x=Math.random(),this.y=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y}},zt=class n{constructor(t,e,i,s,r,o,a,l,h){n.prototype.isMatrix3=!0,this.elements=[1,0,0,0,1,0,0,0,1],t!==void 0&&this.set(t,e,i,s,r,o,a,l,h)}set(t,e,i,s,r,o,a,l,h){let f=this.elements;return f[0]=t,f[1]=s,f[2]=a,f[3]=e,f[4]=r,f[5]=l,f[6]=i,f[7]=o,f[8]=h,this}identity(){return this.set(1,0,0,0,1,0,0,0,1),this}copy(t){let e=this.elements,i=t.elements;return e[0]=i[0],e[1]=i[1],e[2]=i[2],e[3]=i[3],e[4]=i[4],e[5]=i[5],e[6]=i[6],e[7]=i[7],e[8]=i[8],this}extractBasis(t,e,i){return t.setFromMatrix3Column(this,0),e.setFromMatrix3Column(this,1),i.setFromMatrix3Column(this,2),this}setFromMatrix4(t){let e=t.elements;return this.set(e[0],e[4],e[8],e[1],e[5],e[9],e[2],e[6],e[10]),this}multiply(t){return this.multiplyMatrices(this,t)}premultiply(t){return this.multiplyMatrices(t,this)}multiplyMatrices(t,e){let i=t.elements,s=e.elements,r=this.elements,o=i[0],a=i[3],l=i[6],h=i[1],f=i[4],d=i[7],c=i[2],u=i[5],g=i[8],y=s[0],p=s[3],m=s[6],w=s[1],_=s[4],A=s[7],I=s[2],C=s[5],E=s[8];return r[0]=o*y+a*w+l*I,r[3]=o*p+a*_+l*C,r[6]=o*m+a*A+l*E,r[1]=h*y+f*w+d*I,r[4]=h*p+f*_+d*C,r[7]=h*m+f*A+d*E,r[2]=c*y+u*w+g*I,r[5]=c*p+u*_+g*C,r[8]=c*m+u*A+g*E,this}multiplyScalar(t){let e=this.elements;return e[0]*=t,e[3]*=t,e[6]*=t,e[1]*=t,e[4]*=t,e[7]*=t,e[2]*=t,e[5]*=t,e[8]*=t,this}determinant(){let t=this.elements,e=t[0],i=t[1],s=t[2],r=t[3],o=t[4],a=t[5],l=t[6],h=t[7],f=t[8];return e*o*f-e*a*h-i*r*f+i*a*l+s*r*h-s*o*l}invert(){let t=this.elements,e=t[0],i=t[1],s=t[2],r=t[3],o=t[4],a=t[5],l=t[6],h=t[7],f=t[8],d=f*o-a*h,c=a*l-f*r,u=h*r-o*l,g=e*d+i*c+s*u;if(g===0)return this.set(0,0,0,0,0,0,0,0,0);let y=1/g;return t[0]=d*y,t[1]=(s*h-f*i)*y,t[2]=(a*i-s*o)*y,t[3]=c*y,t[4]=(f*e-s*l)*y,t[5]=(s*r-a*e)*y,t[6]=u*y,t[7]=(i*l-h*e)*y,t[8]=(o*e-i*r)*y,this}transpose(){let t,e=this.elements;return t=e[1],e[1]=e[3],e[3]=t,t=e[2],e[2]=e[6],e[6]=t,t=e[5],e[5]=e[7],e[7]=t,this}getNormalMatrix(t){return this.setFromMatrix4(t).invert().transpose()}transposeIntoArray(t){let e=this.elements;return t[0]=e[0],t[1]=e[3],t[2]=e[6],t[3]=e[1],t[4]=e[4],t[5]=e[7],t[6]=e[2],t[7]=e[5],t[8]=e[8],this}setUvTransform(t,e,i,s,r,o,a){let l=Math.cos(r),h=Math.sin(r);return this.set(i*l,i*h,-i*(l*o+h*a)+o+t,-s*h,s*l,-s*(-h*o+l*a)+a+e,0,0,1),this}scale(t,e){return this.premultiply(pa.makeScale(t,e)),this}rotate(t){return this.premultiply(pa.makeRotation(-t)),this}translate(t,e){return this.premultiply(pa.makeTranslation(t,e)),this}makeTranslation(t,e){return t.isVector2?this.set(1,0,t.x,0,1,t.y,0,0,1):this.set(1,0,t,0,1,e,0,0,1),this}makeRotation(t){let e=Math.cos(t),i=Math.sin(t);return this.set(e,-i,0,i,e,0,0,0,1),this}makeScale(t,e){return this.set(t,0,0,0,e,0,0,0,1),this}equals(t){let e=this.elements,i=t.elements;for(let s=0;s<9;s++)if(e[s]!==i[s])return!1;return!0}fromArray(t,e=0){for(let i=0;i<9;i++)this.elements[i]=t[i+e];return this}toArray(t=[],e=0){let i=this.elements;return t[e]=i[0],t[e+1]=i[1],t[e+2]=i[2],t[e+3]=i[3],t[e+4]=i[4],t[e+5]=i[5],t[e+6]=i[6],t[e+7]=i[7],t[e+8]=i[8],t}clone(){return new this.constructor().fromArray(this.elements)}},pa=new zt;function Su(n){for(let t=n.length-1;t>=0;--t)if(n[t]>=65535)return!0;return!1}function to(n){return document.createElementNS("http://www.w3.org/1999/xhtml",n)}function Bf(){let n=to("canvas");return n.style.display="block",n}var mh={};function Zr(n){n in mh||(mh[n]=!0,console.warn(n))}function zf(n,t,e){return new Promise(function(i,s){function r(){switch(n.clientWaitSync(t,n.SYNC_FLUSH_COMMANDS_BIT,0)){case n.WAIT_FAILED:s();break;case n.TIMEOUT_EXPIRED:setTimeout(r,e);break;default:i()}}setTimeout(r,e)})}function kf(n){let t=n.elements;t[2]=.5*t[2]+.5*t[3],t[6]=.5*t[6]+.5*t[7],t[10]=.5*t[10]+.5*t[11],t[14]=.5*t[14]+.5*t[15]}function Hf(n){let t=n.elements;t[11]===-1?(t[10]=-t[10]-1,t[14]=-t[14]):(t[10]=-t[10],t[14]=-t[14]+1)}var gh=new zt().set(.8224621,.177538,0,.0331941,.9668058,0,.0170827,.0723974,.9105199),xh=new zt().set(1.2249401,-.2249404,0,-.0420569,1.0420571,0,-.0196376,-.0786361,1.0982735),Hs={[Kn]:{transfer:Kr,primaries:Jr,luminanceCoefficients:[.2126,.7152,.0722],toReference:n=>n,fromReference:n=>n},[mn]:{transfer:ee,primaries:Jr,luminanceCoefficients:[.2126,.7152,.0722],toReference:n=>n.convertSRGBToLinear(),fromReference:n=>n.convertLinearToSRGB()},[Ao]:{transfer:Kr,primaries:Qr,luminanceCoefficients:[.2289,.6917,.0793],toReference:n=>n.applyMatrix3(xh),fromReference:n=>n.applyMatrix3(gh)},[dc]:{transfer:ee,primaries:Qr,luminanceCoefficients:[.2289,.6917,.0793],toReference:n=>n.convertSRGBToLinear().applyMatrix3(xh),fromReference:n=>n.applyMatrix3(gh).convertLinearToSRGB()}},Vf=new Set([Kn,Ao]),Jt={enabled:!0,_workingColorSpace:Kn,get workingColorSpace(){return this._workingColorSpace},set workingColorSpace(n){if(!Vf.has(n))throw new Error(`Unsupported working color space, "${n}".`);this._workingColorSpace=n},convert:function(n,t,e){if(this.enabled===!1||t===e||!t||!e)return n;let i=Hs[t].toReference,s=Hs[e].fromReference;return s(i(n))},fromWorkingColorSpace:function(n,t){return this.convert(n,this._workingColorSpace,t)},toWorkingColorSpace:function(n,t){return this.convert(n,t,this._workingColorSpace)},getPrimaries:function(n){return Hs[n].primaries},getTransfer:function(n){return n===qn?Kr:Hs[n].transfer},getLuminanceCoefficients:function(n,t=this._workingColorSpace){return n.fromArray(Hs[t].luminanceCoefficients)}};function rs(n){return n<.04045?n*.0773993808:Math.pow(n*.9478672986+.0521327014,2.4)}function ma(n){return n<.0031308?n*12.92:1.055*Math.pow(n,.41666)-.055}var Hi,Cl=class{static getDataURL(t){if(/^data:/i.test(t.src)||typeof HTMLCanvasElement>"u")return t.src;let e;if(t instanceof HTMLCanvasElement)e=t;else{Hi===void 0&&(Hi=to("canvas")),Hi.width=t.width,Hi.height=t.height;let i=Hi.getContext("2d");t instanceof ImageData?i.putImageData(t,0,0):i.drawImage(t,0,0,t.width,t.height),e=Hi}return e.width>2048||e.height>2048?(console.warn("THREE.ImageUtils.getDataURL: Image converted to jpg for performance reasons",t),e.toDataURL("image/jpeg",.6)):e.toDataURL("image/png")}static sRGBToLinear(t){if(typeof HTMLImageElement<"u"&&t instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&t instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&t instanceof ImageBitmap){let e=to("canvas");e.width=t.width,e.height=t.height;let i=e.getContext("2d");i.drawImage(t,0,0,t.width,t.height);let s=i.getImageData(0,0,t.width,t.height),r=s.data;for(let o=0;o<r.length;o++)r[o]=rs(r[o]/255)*255;return i.putImageData(s,0,0),e}else if(t.data){let e=t.data.slice(0);for(let i=0;i<e.length;i++)e instanceof Uint8Array||e instanceof Uint8ClampedArray?e[i]=Math.floor(rs(e[i]/255)*255):e[i]=rs(e[i]);return{data:e,width:t.width,height:t.height}}else return console.warn("THREE.ImageUtils.sRGBToLinear(): Unsupported image type. No color space conversion applied."),t}},Gf=0,eo=class{constructor(t=null){this.isSource=!0,Object.defineProperty(this,"id",{value:Gf++}),this.uuid=xs(),this.data=t,this.dataReady=!0,this.version=0}set needsUpdate(t){t===!0&&this.version++}toJSON(t){let e=t===void 0||typeof t=="string";if(!e&&t.images[this.uuid]!==void 0)return t.images[this.uuid];let i={uuid:this.uuid,url:""},s=this.data;if(s!==null){let r;if(Array.isArray(s)){r=[];for(let o=0,a=s.length;o<a;o++)s[o].isDataTexture?r.push(ga(s[o].image)):r.push(ga(s[o]))}else r=ga(s);i.url=r}return e||(t.images[this.uuid]=i),i}};function ga(n){return typeof HTMLImageElement<"u"&&n instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&n instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&n instanceof ImageBitmap?Cl.getDataURL(n):n.data?{data:Array.from(n.data),width:n.width,height:n.height,type:n.data.constructor.name}:(console.warn("THREE.Texture: Unable to serialize Texture."),{})}var Wf=0,Ce=class n extends Ln{constructor(t=n.DEFAULT_IMAGE,e=n.DEFAULT_MAPPING,i=gi,s=gi,r=Xe,o=xi,a=hn,l=In,h=n.DEFAULT_ANISOTROPY,f=qn){super(),this.isTexture=!0,Object.defineProperty(this,"id",{value:Wf++}),this.uuid=xs(),this.name="",this.source=new eo(t),this.mipmaps=[],this.mapping=e,this.channel=0,this.wrapS=i,this.wrapT=s,this.magFilter=r,this.minFilter=o,this.anisotropy=h,this.format=a,this.internalFormat=null,this.type=l,this.offset=new yt(0,0),this.repeat=new yt(1,1),this.center=new yt(0,0),this.rotation=0,this.matrixAutoUpdate=!0,this.matrix=new zt,this.generateMipmaps=!0,this.premultiplyAlpha=!1,this.flipY=!0,this.unpackAlignment=4,this.colorSpace=f,this.userData={},this.version=0,this.onUpdate=null,this.isRenderTargetTexture=!1,this.pmremVersion=0}get image(){return this.source.data}set image(t=null){this.source.data=t}updateMatrix(){this.matrix.setUvTransform(this.offset.x,this.offset.y,this.repeat.x,this.repeat.y,this.rotation,this.center.x,this.center.y)}clone(){return new this.constructor().copy(this)}copy(t){return this.name=t.name,this.source=t.source,this.mipmaps=t.mipmaps.slice(0),this.mapping=t.mapping,this.channel=t.channel,this.wrapS=t.wrapS,this.wrapT=t.wrapT,this.magFilter=t.magFilter,this.minFilter=t.minFilter,this.anisotropy=t.anisotropy,this.format=t.format,this.internalFormat=t.internalFormat,this.type=t.type,this.offset.copy(t.offset),this.repeat.copy(t.repeat),this.center.copy(t.center),this.rotation=t.rotation,this.matrixAutoUpdate=t.matrixAutoUpdate,this.matrix.copy(t.matrix),this.generateMipmaps=t.generateMipmaps,this.premultiplyAlpha=t.premultiplyAlpha,this.flipY=t.flipY,this.unpackAlignment=t.unpackAlignment,this.colorSpace=t.colorSpace,this.userData=JSON.parse(JSON.stringify(t.userData)),this.needsUpdate=!0,this}toJSON(t){let e=t===void 0||typeof t=="string";if(!e&&t.textures[this.uuid]!==void 0)return t.textures[this.uuid];let i={metadata:{version:4.6,type:"Texture",generator:"Texture.toJSON"},uuid:this.uuid,name:this.name,image:this.source.toJSON(t).uuid,mapping:this.mapping,channel:this.channel,repeat:[this.repeat.x,this.repeat.y],offset:[this.offset.x,this.offset.y],center:[this.center.x,this.center.y],rotation:this.rotation,wrap:[this.wrapS,this.wrapT],format:this.format,internalFormat:this.internalFormat,type:this.type,colorSpace:this.colorSpace,minFilter:this.minFilter,magFilter:this.magFilter,anisotropy:this.anisotropy,flipY:this.flipY,generateMipmaps:this.generateMipmaps,premultiplyAlpha:this.premultiplyAlpha,unpackAlignment:this.unpackAlignment};return Object.keys(this.userData).length>0&&(i.userData=this.userData),e||(t.textures[this.uuid]=i),i}dispose(){this.dispatchEvent({type:"dispose"})}transformUv(t){if(this.mapping!==hu)return t;if(t.applyMatrix3(this.matrix),t.x<0||t.x>1)switch(this.wrapS){case $a:t.x=t.x-Math.floor(t.x);break;case gi:t.x=t.x<0?0:1;break;case tl:Math.abs(Math.floor(t.x)%2)===1?t.x=Math.ceil(t.x)-t.x:t.x=t.x-Math.floor(t.x);break}if(t.y<0||t.y>1)switch(this.wrapT){case $a:t.y=t.y-Math.floor(t.y);break;case gi:t.y=t.y<0?0:1;break;case tl:Math.abs(Math.floor(t.y)%2)===1?t.y=Math.ceil(t.y)-t.y:t.y=t.y-Math.floor(t.y);break}return this.flipY&&(t.y=1-t.y),t}set needsUpdate(t){t===!0&&(this.version++,this.source.needsUpdate=!0)}set needsPMREMUpdate(t){t===!0&&this.pmremVersion++}};Ce.DEFAULT_IMAGE=null;Ce.DEFAULT_MAPPING=hu;Ce.DEFAULT_ANISOTROPY=1;var oe=class n{constructor(t=0,e=0,i=0,s=1){n.prototype.isVector4=!0,this.x=t,this.y=e,this.z=i,this.w=s}get width(){return this.z}set width(t){this.z=t}get height(){return this.w}set height(t){this.w=t}set(t,e,i,s){return this.x=t,this.y=e,this.z=i,this.w=s,this}setScalar(t){return this.x=t,this.y=t,this.z=t,this.w=t,this}setX(t){return this.x=t,this}setY(t){return this.y=t,this}setZ(t){return this.z=t,this}setW(t){return this.w=t,this}setComponent(t,e){switch(t){case 0:this.x=e;break;case 1:this.y=e;break;case 2:this.z=e;break;case 3:this.w=e;break;default:throw new Error("index is out of range: "+t)}return this}getComponent(t){switch(t){case 0:return this.x;case 1:return this.y;case 2:return this.z;case 3:return this.w;default:throw new Error("index is out of range: "+t)}}clone(){return new this.constructor(this.x,this.y,this.z,this.w)}copy(t){return this.x=t.x,this.y=t.y,this.z=t.z,this.w=t.w!==void 0?t.w:1,this}add(t){return this.x+=t.x,this.y+=t.y,this.z+=t.z,this.w+=t.w,this}addScalar(t){return this.x+=t,this.y+=t,this.z+=t,this.w+=t,this}addVectors(t,e){return this.x=t.x+e.x,this.y=t.y+e.y,this.z=t.z+e.z,this.w=t.w+e.w,this}addScaledVector(t,e){return this.x+=t.x*e,this.y+=t.y*e,this.z+=t.z*e,this.w+=t.w*e,this}sub(t){return this.x-=t.x,this.y-=t.y,this.z-=t.z,this.w-=t.w,this}subScalar(t){return this.x-=t,this.y-=t,this.z-=t,this.w-=t,this}subVectors(t,e){return this.x=t.x-e.x,this.y=t.y-e.y,this.z=t.z-e.z,this.w=t.w-e.w,this}multiply(t){return this.x*=t.x,this.y*=t.y,this.z*=t.z,this.w*=t.w,this}multiplyScalar(t){return this.x*=t,this.y*=t,this.z*=t,this.w*=t,this}applyMatrix4(t){let e=this.x,i=this.y,s=this.z,r=this.w,o=t.elements;return this.x=o[0]*e+o[4]*i+o[8]*s+o[12]*r,this.y=o[1]*e+o[5]*i+o[9]*s+o[13]*r,this.z=o[2]*e+o[6]*i+o[10]*s+o[14]*r,this.w=o[3]*e+o[7]*i+o[11]*s+o[15]*r,this}divideScalar(t){return this.multiplyScalar(1/t)}setAxisAngleFromQuaternion(t){this.w=2*Math.acos(t.w);let e=Math.sqrt(1-t.w*t.w);return e<1e-4?(this.x=1,this.y=0,this.z=0):(this.x=t.x/e,this.y=t.y/e,this.z=t.z/e),this}setAxisAngleFromRotationMatrix(t){let e,i,s,r,l=t.elements,h=l[0],f=l[4],d=l[8],c=l[1],u=l[5],g=l[9],y=l[2],p=l[6],m=l[10];if(Math.abs(f-c)<.01&&Math.abs(d-y)<.01&&Math.abs(g-p)<.01){if(Math.abs(f+c)<.1&&Math.abs(d+y)<.1&&Math.abs(g+p)<.1&&Math.abs(h+u+m-3)<.1)return this.set(1,0,0,0),this;e=Math.PI;let _=(h+1)/2,A=(u+1)/2,I=(m+1)/2,C=(f+c)/4,E=(d+y)/4,T=(g+p)/4;return _>A&&_>I?_<.01?(i=0,s=.707106781,r=.707106781):(i=Math.sqrt(_),s=C/i,r=E/i):A>I?A<.01?(i=.707106781,s=0,r=.707106781):(s=Math.sqrt(A),i=C/s,r=T/s):I<.01?(i=.707106781,s=.707106781,r=0):(r=Math.sqrt(I),i=E/r,s=T/r),this.set(i,s,r,e),this}let w=Math.sqrt((p-g)*(p-g)+(d-y)*(d-y)+(c-f)*(c-f));return Math.abs(w)<.001&&(w=1),this.x=(p-g)/w,this.y=(d-y)/w,this.z=(c-f)/w,this.w=Math.acos((h+u+m-1)/2),this}setFromMatrixPosition(t){let e=t.elements;return this.x=e[12],this.y=e[13],this.z=e[14],this.w=e[15],this}min(t){return this.x=Math.min(this.x,t.x),this.y=Math.min(this.y,t.y),this.z=Math.min(this.z,t.z),this.w=Math.min(this.w,t.w),this}max(t){return this.x=Math.max(this.x,t.x),this.y=Math.max(this.y,t.y),this.z=Math.max(this.z,t.z),this.w=Math.max(this.w,t.w),this}clamp(t,e){return this.x=Math.max(t.x,Math.min(e.x,this.x)),this.y=Math.max(t.y,Math.min(e.y,this.y)),this.z=Math.max(t.z,Math.min(e.z,this.z)),this.w=Math.max(t.w,Math.min(e.w,this.w)),this}clampScalar(t,e){return this.x=Math.max(t,Math.min(e,this.x)),this.y=Math.max(t,Math.min(e,this.y)),this.z=Math.max(t,Math.min(e,this.z)),this.w=Math.max(t,Math.min(e,this.w)),this}clampLength(t,e){let i=this.length();return this.divideScalar(i||1).multiplyScalar(Math.max(t,Math.min(e,i)))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this.w=Math.floor(this.w),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this.w=Math.ceil(this.w),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this.w=Math.round(this.w),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this.w=Math.trunc(this.w),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this.w=-this.w,this}dot(t){return this.x*t.x+this.y*t.y+this.z*t.z+this.w*t.w}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)+Math.abs(this.w)}normalize(){return this.divideScalar(this.length()||1)}setLength(t){return this.normalize().multiplyScalar(t)}lerp(t,e){return this.x+=(t.x-this.x)*e,this.y+=(t.y-this.y)*e,this.z+=(t.z-this.z)*e,this.w+=(t.w-this.w)*e,this}lerpVectors(t,e,i){return this.x=t.x+(e.x-t.x)*i,this.y=t.y+(e.y-t.y)*i,this.z=t.z+(e.z-t.z)*i,this.w=t.w+(e.w-t.w)*i,this}equals(t){return t.x===this.x&&t.y===this.y&&t.z===this.z&&t.w===this.w}fromArray(t,e=0){return this.x=t[e],this.y=t[e+1],this.z=t[e+2],this.w=t[e+3],this}toArray(t=[],e=0){return t[e]=this.x,t[e+1]=this.y,t[e+2]=this.z,t[e+3]=this.w,t}fromBufferAttribute(t,e){return this.x=t.getX(e),this.y=t.getY(e),this.z=t.getZ(e),this.w=t.getW(e),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this.w=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z,yield this.w}},Rl=class extends Ln{constructor(t=1,e=1,i={}){super(),this.isRenderTarget=!0,this.width=t,this.height=e,this.depth=1,this.scissor=new oe(0,0,t,e),this.scissorTest=!1,this.viewport=new oe(0,0,t,e);let s={width:t,height:e,depth:1};i=Object.assign({generateMipmaps:!1,internalFormat:null,minFilter:Xe,depthBuffer:!0,stencilBuffer:!1,resolveDepthBuffer:!0,resolveStencilBuffer:!0,depthTexture:null,samples:0,count:1},i);let r=new Ce(s,i.mapping,i.wrapS,i.wrapT,i.magFilter,i.minFilter,i.format,i.type,i.anisotropy,i.colorSpace);r.flipY=!1,r.generateMipmaps=i.generateMipmaps,r.internalFormat=i.internalFormat,this.textures=[];let o=i.count;for(let a=0;a<o;a++)this.textures[a]=r.clone(),this.textures[a].isRenderTargetTexture=!0;this.depthBuffer=i.depthBuffer,this.stencilBuffer=i.stencilBuffer,this.resolveDepthBuffer=i.resolveDepthBuffer,this.resolveStencilBuffer=i.resolveStencilBuffer,this.depthTexture=i.depthTexture,this.samples=i.samples}get texture(){return this.textures[0]}set texture(t){this.textures[0]=t}setSize(t,e,i=1){if(this.width!==t||this.height!==e||this.depth!==i){this.width=t,this.height=e,this.depth=i;for(let s=0,r=this.textures.length;s<r;s++)this.textures[s].image.width=t,this.textures[s].image.height=e,this.textures[s].image.depth=i;this.dispose()}this.viewport.set(0,0,t,e),this.scissor.set(0,0,t,e)}clone(){return new this.constructor().copy(this)}copy(t){this.width=t.width,this.height=t.height,this.depth=t.depth,this.scissor.copy(t.scissor),this.scissorTest=t.scissorTest,this.viewport.copy(t.viewport),this.textures.length=0;for(let i=0,s=t.textures.length;i<s;i++)this.textures[i]=t.textures[i].clone(),this.textures[i].isRenderTargetTexture=!0;let e=Object.assign({},t.texture.image);return this.texture.source=new eo(e),this.depthBuffer=t.depthBuffer,this.stencilBuffer=t.stencilBuffer,this.resolveDepthBuffer=t.resolveDepthBuffer,this.resolveStencilBuffer=t.resolveStencilBuffer,t.depthTexture!==null&&(this.depthTexture=t.depthTexture.clone()),this.samples=t.samples,this}dispose(){this.dispatchEvent({type:"dispose"})}},ue=class extends Rl{constructor(t=1,e=1,i={}){super(t,e,i),this.isWebGLRenderTarget=!0}},no=class extends Ce{constructor(t=null,e=1,i=1,s=1){super(null),this.isDataArrayTexture=!0,this.image={data:t,width:e,height:i,depth:s},this.magFilter=be,this.minFilter=be,this.wrapR=gi,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1,this.layerUpdates=new Set}addLayerUpdate(t){this.layerUpdates.add(t)}clearLayerUpdates(){this.layerUpdates.clear()}};var Pl=class extends Ce{constructor(t=null,e=1,i=1,s=1){super(null),this.isData3DTexture=!0,this.image={data:t,width:e,height:i,depth:s},this.magFilter=be,this.minFilter=be,this.wrapR=gi,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}};var $e=class{constructor(t=0,e=0,i=0,s=1){this.isQuaternion=!0,this._x=t,this._y=e,this._z=i,this._w=s}static slerpFlat(t,e,i,s,r,o,a){let l=i[s+0],h=i[s+1],f=i[s+2],d=i[s+3],c=r[o+0],u=r[o+1],g=r[o+2],y=r[o+3];if(a===0){t[e+0]=l,t[e+1]=h,t[e+2]=f,t[e+3]=d;return}if(a===1){t[e+0]=c,t[e+1]=u,t[e+2]=g,t[e+3]=y;return}if(d!==y||l!==c||h!==u||f!==g){let p=1-a,m=l*c+h*u+f*g+d*y,w=m>=0?1:-1,_=1-m*m;if(_>Number.EPSILON){let I=Math.sqrt(_),C=Math.atan2(I,m*w);p=Math.sin(p*C)/I,a=Math.sin(a*C)/I}let A=a*w;if(l=l*p+c*A,h=h*p+u*A,f=f*p+g*A,d=d*p+y*A,p===1-a){let I=1/Math.sqrt(l*l+h*h+f*f+d*d);l*=I,h*=I,f*=I,d*=I}}t[e]=l,t[e+1]=h,t[e+2]=f,t[e+3]=d}static multiplyQuaternionsFlat(t,e,i,s,r,o){let a=i[s],l=i[s+1],h=i[s+2],f=i[s+3],d=r[o],c=r[o+1],u=r[o+2],g=r[o+3];return t[e]=a*g+f*d+l*u-h*c,t[e+1]=l*g+f*c+h*d-a*u,t[e+2]=h*g+f*u+a*c-l*d,t[e+3]=f*g-a*d-l*c-h*u,t}get x(){return this._x}set x(t){this._x=t,this._onChangeCallback()}get y(){return this._y}set y(t){this._y=t,this._onChangeCallback()}get z(){return this._z}set z(t){this._z=t,this._onChangeCallback()}get w(){return this._w}set w(t){this._w=t,this._onChangeCallback()}set(t,e,i,s){return this._x=t,this._y=e,this._z=i,this._w=s,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._w)}copy(t){return this._x=t.x,this._y=t.y,this._z=t.z,this._w=t.w,this._onChangeCallback(),this}setFromEuler(t,e=!0){let i=t._x,s=t._y,r=t._z,o=t._order,a=Math.cos,l=Math.sin,h=a(i/2),f=a(s/2),d=a(r/2),c=l(i/2),u=l(s/2),g=l(r/2);switch(o){case"XYZ":this._x=c*f*d+h*u*g,this._y=h*u*d-c*f*g,this._z=h*f*g+c*u*d,this._w=h*f*d-c*u*g;break;case"YXZ":this._x=c*f*d+h*u*g,this._y=h*u*d-c*f*g,this._z=h*f*g-c*u*d,this._w=h*f*d+c*u*g;break;case"ZXY":this._x=c*f*d-h*u*g,this._y=h*u*d+c*f*g,this._z=h*f*g+c*u*d,this._w=h*f*d-c*u*g;break;case"ZYX":this._x=c*f*d-h*u*g,this._y=h*u*d+c*f*g,this._z=h*f*g-c*u*d,this._w=h*f*d+c*u*g;break;case"YZX":this._x=c*f*d+h*u*g,this._y=h*u*d+c*f*g,this._z=h*f*g-c*u*d,this._w=h*f*d-c*u*g;break;case"XZY":this._x=c*f*d-h*u*g,this._y=h*u*d-c*f*g,this._z=h*f*g+c*u*d,this._w=h*f*d+c*u*g;break;default:console.warn("THREE.Quaternion: .setFromEuler() encountered an unknown order: "+o)}return e===!0&&this._onChangeCallback(),this}setFromAxisAngle(t,e){let i=e/2,s=Math.sin(i);return this._x=t.x*s,this._y=t.y*s,this._z=t.z*s,this._w=Math.cos(i),this._onChangeCallback(),this}setFromRotationMatrix(t){let e=t.elements,i=e[0],s=e[4],r=e[8],o=e[1],a=e[5],l=e[9],h=e[2],f=e[6],d=e[10],c=i+a+d;if(c>0){let u=.5/Math.sqrt(c+1);this._w=.25/u,this._x=(f-l)*u,this._y=(r-h)*u,this._z=(o-s)*u}else if(i>a&&i>d){let u=2*Math.sqrt(1+i-a-d);this._w=(f-l)/u,this._x=.25*u,this._y=(s+o)/u,this._z=(r+h)/u}else if(a>d){let u=2*Math.sqrt(1+a-i-d);this._w=(r-h)/u,this._x=(s+o)/u,this._y=.25*u,this._z=(l+f)/u}else{let u=2*Math.sqrt(1+d-i-a);this._w=(o-s)/u,this._x=(r+h)/u,this._y=(l+f)/u,this._z=.25*u}return this._onChangeCallback(),this}setFromUnitVectors(t,e){let i=t.dot(e)+1;return i<Number.EPSILON?(i=0,Math.abs(t.x)>Math.abs(t.z)?(this._x=-t.y,this._y=t.x,this._z=0,this._w=i):(this._x=0,this._y=-t.z,this._z=t.y,this._w=i)):(this._x=t.y*e.z-t.z*e.y,this._y=t.z*e.x-t.x*e.z,this._z=t.x*e.y-t.y*e.x,this._w=i),this.normalize()}angleTo(t){return 2*Math.acos(Math.abs(Le(this.dot(t),-1,1)))}rotateTowards(t,e){let i=this.angleTo(t);if(i===0)return this;let s=Math.min(1,e/i);return this.slerp(t,s),this}identity(){return this.set(0,0,0,1)}invert(){return this.conjugate()}conjugate(){return this._x*=-1,this._y*=-1,this._z*=-1,this._onChangeCallback(),this}dot(t){return this._x*t._x+this._y*t._y+this._z*t._z+this._w*t._w}lengthSq(){return this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w}length(){return Math.sqrt(this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w)}normalize(){let t=this.length();return t===0?(this._x=0,this._y=0,this._z=0,this._w=1):(t=1/t,this._x=this._x*t,this._y=this._y*t,this._z=this._z*t,this._w=this._w*t),this._onChangeCallback(),this}multiply(t){return this.multiplyQuaternions(this,t)}premultiply(t){return this.multiplyQuaternions(t,this)}multiplyQuaternions(t,e){let i=t._x,s=t._y,r=t._z,o=t._w,a=e._x,l=e._y,h=e._z,f=e._w;return this._x=i*f+o*a+s*h-r*l,this._y=s*f+o*l+r*a-i*h,this._z=r*f+o*h+i*l-s*a,this._w=o*f-i*a-s*l-r*h,this._onChangeCallback(),this}slerp(t,e){if(e===0)return this;if(e===1)return this.copy(t);let i=this._x,s=this._y,r=this._z,o=this._w,a=o*t._w+i*t._x+s*t._y+r*t._z;if(a<0?(this._w=-t._w,this._x=-t._x,this._y=-t._y,this._z=-t._z,a=-a):this.copy(t),a>=1)return this._w=o,this._x=i,this._y=s,this._z=r,this;let l=1-a*a;if(l<=Number.EPSILON){let u=1-e;return this._w=u*o+e*this._w,this._x=u*i+e*this._x,this._y=u*s+e*this._y,this._z=u*r+e*this._z,this.normalize(),this}let h=Math.sqrt(l),f=Math.atan2(h,a),d=Math.sin((1-e)*f)/h,c=Math.sin(e*f)/h;return this._w=o*d+this._w*c,this._x=i*d+this._x*c,this._y=s*d+this._y*c,this._z=r*d+this._z*c,this._onChangeCallback(),this}slerpQuaternions(t,e,i){return this.copy(t).slerp(e,i)}random(){let t=2*Math.PI*Math.random(),e=2*Math.PI*Math.random(),i=Math.random(),s=Math.sqrt(1-i),r=Math.sqrt(i);return this.set(s*Math.sin(t),s*Math.cos(t),r*Math.sin(e),r*Math.cos(e))}equals(t){return t._x===this._x&&t._y===this._y&&t._z===this._z&&t._w===this._w}fromArray(t,e=0){return this._x=t[e],this._y=t[e+1],this._z=t[e+2],this._w=t[e+3],this._onChangeCallback(),this}toArray(t=[],e=0){return t[e]=this._x,t[e+1]=this._y,t[e+2]=this._z,t[e+3]=this._w,t}fromBufferAttribute(t,e){return this._x=t.getX(e),this._y=t.getY(e),this._z=t.getZ(e),this._w=t.getW(e),this._onChangeCallback(),this}toJSON(){return this.toArray()}_onChange(t){return this._onChangeCallback=t,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._w}},U=class n{constructor(t=0,e=0,i=0){n.prototype.isVector3=!0,this.x=t,this.y=e,this.z=i}set(t,e,i){return i===void 0&&(i=this.z),this.x=t,this.y=e,this.z=i,this}setScalar(t){return this.x=t,this.y=t,this.z=t,this}setX(t){return this.x=t,this}setY(t){return this.y=t,this}setZ(t){return this.z=t,this}setComponent(t,e){switch(t){case 0:this.x=e;break;case 1:this.y=e;break;case 2:this.z=e;break;default:throw new Error("index is out of range: "+t)}return this}getComponent(t){switch(t){case 0:return this.x;case 1:return this.y;case 2:return this.z;default:throw new Error("index is out of range: "+t)}}clone(){return new this.constructor(this.x,this.y,this.z)}copy(t){return this.x=t.x,this.y=t.y,this.z=t.z,this}add(t){return this.x+=t.x,this.y+=t.y,this.z+=t.z,this}addScalar(t){return this.x+=t,this.y+=t,this.z+=t,this}addVectors(t,e){return this.x=t.x+e.x,this.y=t.y+e.y,this.z=t.z+e.z,this}addScaledVector(t,e){return this.x+=t.x*e,this.y+=t.y*e,this.z+=t.z*e,this}sub(t){return this.x-=t.x,this.y-=t.y,this.z-=t.z,this}subScalar(t){return this.x-=t,this.y-=t,this.z-=t,this}subVectors(t,e){return this.x=t.x-e.x,this.y=t.y-e.y,this.z=t.z-e.z,this}multiply(t){return this.x*=t.x,this.y*=t.y,this.z*=t.z,this}multiplyScalar(t){return this.x*=t,this.y*=t,this.z*=t,this}multiplyVectors(t,e){return this.x=t.x*e.x,this.y=t.y*e.y,this.z=t.z*e.z,this}applyEuler(t){return this.applyQuaternion(vh.setFromEuler(t))}applyAxisAngle(t,e){return this.applyQuaternion(vh.setFromAxisAngle(t,e))}applyMatrix3(t){let e=this.x,i=this.y,s=this.z,r=t.elements;return this.x=r[0]*e+r[3]*i+r[6]*s,this.y=r[1]*e+r[4]*i+r[7]*s,this.z=r[2]*e+r[5]*i+r[8]*s,this}applyNormalMatrix(t){return this.applyMatrix3(t).normalize()}applyMatrix4(t){let e=this.x,i=this.y,s=this.z,r=t.elements,o=1/(r[3]*e+r[7]*i+r[11]*s+r[15]);return this.x=(r[0]*e+r[4]*i+r[8]*s+r[12])*o,this.y=(r[1]*e+r[5]*i+r[9]*s+r[13])*o,this.z=(r[2]*e+r[6]*i+r[10]*s+r[14])*o,this}applyQuaternion(t){let e=this.x,i=this.y,s=this.z,r=t.x,o=t.y,a=t.z,l=t.w,h=2*(o*s-a*i),f=2*(a*e-r*s),d=2*(r*i-o*e);return this.x=e+l*h+o*d-a*f,this.y=i+l*f+a*h-r*d,this.z=s+l*d+r*f-o*h,this}project(t){return this.applyMatrix4(t.matrixWorldInverse).applyMatrix4(t.projectionMatrix)}unproject(t){return this.applyMatrix4(t.projectionMatrixInverse).applyMatrix4(t.matrixWorld)}transformDirection(t){let e=this.x,i=this.y,s=this.z,r=t.elements;return this.x=r[0]*e+r[4]*i+r[8]*s,this.y=r[1]*e+r[5]*i+r[9]*s,this.z=r[2]*e+r[6]*i+r[10]*s,this.normalize()}divide(t){return this.x/=t.x,this.y/=t.y,this.z/=t.z,this}divideScalar(t){return this.multiplyScalar(1/t)}min(t){return this.x=Math.min(this.x,t.x),this.y=Math.min(this.y,t.y),this.z=Math.min(this.z,t.z),this}max(t){return this.x=Math.max(this.x,t.x),this.y=Math.max(this.y,t.y),this.z=Math.max(this.z,t.z),this}clamp(t,e){return this.x=Math.max(t.x,Math.min(e.x,this.x)),this.y=Math.max(t.y,Math.min(e.y,this.y)),this.z=Math.max(t.z,Math.min(e.z,this.z)),this}clampScalar(t,e){return this.x=Math.max(t,Math.min(e,this.x)),this.y=Math.max(t,Math.min(e,this.y)),this.z=Math.max(t,Math.min(e,this.z)),this}clampLength(t,e){let i=this.length();return this.divideScalar(i||1).multiplyScalar(Math.max(t,Math.min(e,i)))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this}dot(t){return this.x*t.x+this.y*t.y+this.z*t.z}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)}normalize(){return this.divideScalar(this.length()||1)}setLength(t){return this.normalize().multiplyScalar(t)}lerp(t,e){return this.x+=(t.x-this.x)*e,this.y+=(t.y-this.y)*e,this.z+=(t.z-this.z)*e,this}lerpVectors(t,e,i){return this.x=t.x+(e.x-t.x)*i,this.y=t.y+(e.y-t.y)*i,this.z=t.z+(e.z-t.z)*i,this}cross(t){return this.crossVectors(this,t)}crossVectors(t,e){let i=t.x,s=t.y,r=t.z,o=e.x,a=e.y,l=e.z;return this.x=s*l-r*a,this.y=r*o-i*l,this.z=i*a-s*o,this}projectOnVector(t){let e=t.lengthSq();if(e===0)return this.set(0,0,0);let i=t.dot(this)/e;return this.copy(t).multiplyScalar(i)}projectOnPlane(t){return xa.copy(this).projectOnVector(t),this.sub(xa)}reflect(t){return this.sub(xa.copy(t).multiplyScalar(2*this.dot(t)))}angleTo(t){let e=Math.sqrt(this.lengthSq()*t.lengthSq());if(e===0)return Math.PI/2;let i=this.dot(t)/e;return Math.acos(Le(i,-1,1))}distanceTo(t){return Math.sqrt(this.distanceToSquared(t))}distanceToSquared(t){let e=this.x-t.x,i=this.y-t.y,s=this.z-t.z;return e*e+i*i+s*s}manhattanDistanceTo(t){return Math.abs(this.x-t.x)+Math.abs(this.y-t.y)+Math.abs(this.z-t.z)}setFromSpherical(t){return this.setFromSphericalCoords(t.radius,t.phi,t.theta)}setFromSphericalCoords(t,e,i){let s=Math.sin(e)*t;return this.x=s*Math.sin(i),this.y=Math.cos(e)*t,this.z=s*Math.cos(i),this}setFromCylindrical(t){return this.setFromCylindricalCoords(t.radius,t.theta,t.y)}setFromCylindricalCoords(t,e,i){return this.x=t*Math.sin(e),this.y=i,this.z=t*Math.cos(e),this}setFromMatrixPosition(t){let e=t.elements;return this.x=e[12],this.y=e[13],this.z=e[14],this}setFromMatrixScale(t){let e=this.setFromMatrixColumn(t,0).length(),i=this.setFromMatrixColumn(t,1).length(),s=this.setFromMatrixColumn(t,2).length();return this.x=e,this.y=i,this.z=s,this}setFromMatrixColumn(t,e){return this.fromArray(t.elements,e*4)}setFromMatrix3Column(t,e){return this.fromArray(t.elements,e*3)}setFromEuler(t){return this.x=t._x,this.y=t._y,this.z=t._z,this}setFromColor(t){return this.x=t.r,this.y=t.g,this.z=t.b,this}equals(t){return t.x===this.x&&t.y===this.y&&t.z===this.z}fromArray(t,e=0){return this.x=t[e],this.y=t[e+1],this.z=t[e+2],this}toArray(t=[],e=0){return t[e]=this.x,t[e+1]=this.y,t[e+2]=this.z,t}fromBufferAttribute(t,e){return this.x=t.getX(e),this.y=t.getY(e),this.z=t.getZ(e),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this}randomDirection(){let t=Math.random()*Math.PI*2,e=Math.random()*2-1,i=Math.sqrt(1-e*e);return this.x=i*Math.cos(t),this.y=e,this.z=i*Math.sin(t),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z}},xa=new U,vh=new $e,Dn=class{constructor(t=new U(1/0,1/0,1/0),e=new U(-1/0,-1/0,-1/0)){this.isBox3=!0,this.min=t,this.max=e}set(t,e){return this.min.copy(t),this.max.copy(e),this}setFromArray(t){this.makeEmpty();for(let e=0,i=t.length;e<i;e+=3)this.expandByPoint(on.fromArray(t,e));return this}setFromBufferAttribute(t){this.makeEmpty();for(let e=0,i=t.count;e<i;e++)this.expandByPoint(on.fromBufferAttribute(t,e));return this}setFromPoints(t){this.makeEmpty();for(let e=0,i=t.length;e<i;e++)this.expandByPoint(t[e]);return this}setFromCenterAndSize(t,e){let i=on.copy(e).multiplyScalar(.5);return this.min.copy(t).sub(i),this.max.copy(t).add(i),this}setFromObject(t,e=!1){return this.makeEmpty(),this.expandByObject(t,e)}clone(){return new this.constructor().copy(this)}copy(t){return this.min.copy(t.min),this.max.copy(t.max),this}makeEmpty(){return this.min.x=this.min.y=this.min.z=1/0,this.max.x=this.max.y=this.max.z=-1/0,this}isEmpty(){return this.max.x<this.min.x||this.max.y<this.min.y||this.max.z<this.min.z}getCenter(t){return this.isEmpty()?t.set(0,0,0):t.addVectors(this.min,this.max).multiplyScalar(.5)}getSize(t){return this.isEmpty()?t.set(0,0,0):t.subVectors(this.max,this.min)}expandByPoint(t){return this.min.min(t),this.max.max(t),this}expandByVector(t){return this.min.sub(t),this.max.add(t),this}expandByScalar(t){return this.min.addScalar(-t),this.max.addScalar(t),this}expandByObject(t,e=!1){t.updateWorldMatrix(!1,!1);let i=t.geometry;if(i!==void 0){let r=i.getAttribute("position");if(e===!0&&r!==void 0&&t.isInstancedMesh!==!0)for(let o=0,a=r.count;o<a;o++)t.isMesh===!0?t.getVertexPosition(o,on):on.fromBufferAttribute(r,o),on.applyMatrix4(t.matrixWorld),this.expandByPoint(on);else t.boundingBox!==void 0?(t.boundingBox===null&&t.computeBoundingBox(),Mr.copy(t.boundingBox)):(i.boundingBox===null&&i.computeBoundingBox(),Mr.copy(i.boundingBox)),Mr.applyMatrix4(t.matrixWorld),this.union(Mr)}let s=t.children;for(let r=0,o=s.length;r<o;r++)this.expandByObject(s[r],e);return this}containsPoint(t){return t.x>=this.min.x&&t.x<=this.max.x&&t.y>=this.min.y&&t.y<=this.max.y&&t.z>=this.min.z&&t.z<=this.max.z}containsBox(t){return this.min.x<=t.min.x&&t.max.x<=this.max.x&&this.min.y<=t.min.y&&t.max.y<=this.max.y&&this.min.z<=t.min.z&&t.max.z<=this.max.z}getParameter(t,e){return e.set((t.x-this.min.x)/(this.max.x-this.min.x),(t.y-this.min.y)/(this.max.y-this.min.y),(t.z-this.min.z)/(this.max.z-this.min.z))}intersectsBox(t){return t.max.x>=this.min.x&&t.min.x<=this.max.x&&t.max.y>=this.min.y&&t.min.y<=this.max.y&&t.max.z>=this.min.z&&t.min.z<=this.max.z}intersectsSphere(t){return this.clampPoint(t.center,on),on.distanceToSquared(t.center)<=t.radius*t.radius}intersectsPlane(t){let e,i;return t.normal.x>0?(e=t.normal.x*this.min.x,i=t.normal.x*this.max.x):(e=t.normal.x*this.max.x,i=t.normal.x*this.min.x),t.normal.y>0?(e+=t.normal.y*this.min.y,i+=t.normal.y*this.max.y):(e+=t.normal.y*this.max.y,i+=t.normal.y*this.min.y),t.normal.z>0?(e+=t.normal.z*this.min.z,i+=t.normal.z*this.max.z):(e+=t.normal.z*this.max.z,i+=t.normal.z*this.min.z),e<=-t.constant&&i>=-t.constant}intersectsTriangle(t){if(this.isEmpty())return!1;this.getCenter(Vs),Sr.subVectors(this.max,Vs),Vi.subVectors(t.a,Vs),Gi.subVectors(t.b,Vs),Wi.subVectors(t.c,Vs),kn.subVectors(Gi,Vi),Hn.subVectors(Wi,Gi),oi.subVectors(Vi,Wi);let e=[0,-kn.z,kn.y,0,-Hn.z,Hn.y,0,-oi.z,oi.y,kn.z,0,-kn.x,Hn.z,0,-Hn.x,oi.z,0,-oi.x,-kn.y,kn.x,0,-Hn.y,Hn.x,0,-oi.y,oi.x,0];return!va(e,Vi,Gi,Wi,Sr)||(e=[1,0,0,0,1,0,0,0,1],!va(e,Vi,Gi,Wi,Sr))?!1:(br.crossVectors(kn,Hn),e=[br.x,br.y,br.z],va(e,Vi,Gi,Wi,Sr))}clampPoint(t,e){return e.copy(t).clamp(this.min,this.max)}distanceToPoint(t){return this.clampPoint(t,on).distanceTo(t)}getBoundingSphere(t){return this.isEmpty()?t.makeEmpty():(this.getCenter(t.center),t.radius=this.getSize(on).length()*.5),t}intersect(t){return this.min.max(t.min),this.max.min(t.max),this.isEmpty()&&this.makeEmpty(),this}union(t){return this.min.min(t.min),this.max.max(t.max),this}applyMatrix4(t){return this.isEmpty()?this:(bn[0].set(this.min.x,this.min.y,this.min.z).applyMatrix4(t),bn[1].set(this.min.x,this.min.y,this.max.z).applyMatrix4(t),bn[2].set(this.min.x,this.max.y,this.min.z).applyMatrix4(t),bn[3].set(this.min.x,this.max.y,this.max.z).applyMatrix4(t),bn[4].set(this.max.x,this.min.y,this.min.z).applyMatrix4(t),bn[5].set(this.max.x,this.min.y,this.max.z).applyMatrix4(t),bn[6].set(this.max.x,this.max.y,this.min.z).applyMatrix4(t),bn[7].set(this.max.x,this.max.y,this.max.z).applyMatrix4(t),this.setFromPoints(bn),this)}translate(t){return this.min.add(t),this.max.add(t),this}equals(t){return t.min.equals(this.min)&&t.max.equals(this.max)}},bn=[new U,new U,new U,new U,new U,new U,new U,new U],on=new U,Mr=new Dn,Vi=new U,Gi=new U,Wi=new U,kn=new U,Hn=new U,oi=new U,Vs=new U,Sr=new U,br=new U,ai=new U;function va(n,t,e,i,s){for(let r=0,o=n.length-3;r<=o;r+=3){ai.fromArray(n,r);let a=s.x*Math.abs(ai.x)+s.y*Math.abs(ai.y)+s.z*Math.abs(ai.z),l=t.dot(ai),h=e.dot(ai),f=i.dot(ai);if(Math.max(-Math.max(l,h,f),Math.min(l,h,f))>a)return!1}return!0}var Xf=new Dn,Gs=new U,_a=new U,_i=class{constructor(t=new U,e=-1){this.isSphere=!0,this.center=t,this.radius=e}set(t,e){return this.center.copy(t),this.radius=e,this}setFromPoints(t,e){let i=this.center;e!==void 0?i.copy(e):Xf.setFromPoints(t).getCenter(i);let s=0;for(let r=0,o=t.length;r<o;r++)s=Math.max(s,i.distanceToSquared(t[r]));return this.radius=Math.sqrt(s),this}copy(t){return this.center.copy(t.center),this.radius=t.radius,this}isEmpty(){return this.radius<0}makeEmpty(){return this.center.set(0,0,0),this.radius=-1,this}containsPoint(t){return t.distanceToSquared(this.center)<=this.radius*this.radius}distanceToPoint(t){return t.distanceTo(this.center)-this.radius}intersectsSphere(t){let e=this.radius+t.radius;return t.center.distanceToSquared(this.center)<=e*e}intersectsBox(t){return t.intersectsSphere(this)}intersectsPlane(t){return Math.abs(t.distanceToPoint(this.center))<=this.radius}clampPoint(t,e){let i=this.center.distanceToSquared(t);return e.copy(t),i>this.radius*this.radius&&(e.sub(this.center).normalize(),e.multiplyScalar(this.radius).add(this.center)),e}getBoundingBox(t){return this.isEmpty()?(t.makeEmpty(),t):(t.set(this.center,this.center),t.expandByScalar(this.radius),t)}applyMatrix4(t){return this.center.applyMatrix4(t),this.radius=this.radius*t.getMaxScaleOnAxis(),this}translate(t){return this.center.add(t),this}expandByPoint(t){if(this.isEmpty())return this.center.copy(t),this.radius=0,this;Gs.subVectors(t,this.center);let e=Gs.lengthSq();if(e>this.radius*this.radius){let i=Math.sqrt(e),s=(i-this.radius)*.5;this.center.addScaledVector(Gs,s/i),this.radius+=s}return this}union(t){return t.isEmpty()?this:this.isEmpty()?(this.copy(t),this):(this.center.equals(t.center)===!0?this.radius=Math.max(this.radius,t.radius):(_a.subVectors(t.center,this.center).setLength(t.radius),this.expandByPoint(Gs.copy(t.center).add(_a)),this.expandByPoint(Gs.copy(t.center).sub(_a))),this)}equals(t){return t.center.equals(this.center)&&t.radius===this.radius}clone(){return new this.constructor().copy(this)}},wn=new U,ya=new U,wr=new U,Vn=new U,Ma=new U,Ar=new U,Sa=new U,io=class{constructor(t=new U,e=new U(0,0,-1)){this.origin=t,this.direction=e}set(t,e){return this.origin.copy(t),this.direction.copy(e),this}copy(t){return this.origin.copy(t.origin),this.direction.copy(t.direction),this}at(t,e){return e.copy(this.origin).addScaledVector(this.direction,t)}lookAt(t){return this.direction.copy(t).sub(this.origin).normalize(),this}recast(t){return this.origin.copy(this.at(t,wn)),this}closestPointToPoint(t,e){e.subVectors(t,this.origin);let i=e.dot(this.direction);return i<0?e.copy(this.origin):e.copy(this.origin).addScaledVector(this.direction,i)}distanceToPoint(t){return Math.sqrt(this.distanceSqToPoint(t))}distanceSqToPoint(t){let e=wn.subVectors(t,this.origin).dot(this.direction);return e<0?this.origin.distanceToSquared(t):(wn.copy(this.origin).addScaledVector(this.direction,e),wn.distanceToSquared(t))}distanceSqToSegment(t,e,i,s){ya.copy(t).add(e).multiplyScalar(.5),wr.copy(e).sub(t).normalize(),Vn.copy(this.origin).sub(ya);let r=t.distanceTo(e)*.5,o=-this.direction.dot(wr),a=Vn.dot(this.direction),l=-Vn.dot(wr),h=Vn.lengthSq(),f=Math.abs(1-o*o),d,c,u,g;if(f>0)if(d=o*l-a,c=o*a-l,g=r*f,d>=0)if(c>=-g)if(c<=g){let y=1/f;d*=y,c*=y,u=d*(d+o*c+2*a)+c*(o*d+c+2*l)+h}else c=r,d=Math.max(0,-(o*c+a)),u=-d*d+c*(c+2*l)+h;else c=-r,d=Math.max(0,-(o*c+a)),u=-d*d+c*(c+2*l)+h;else c<=-g?(d=Math.max(0,-(-o*r+a)),c=d>0?-r:Math.min(Math.max(-r,-l),r),u=-d*d+c*(c+2*l)+h):c<=g?(d=0,c=Math.min(Math.max(-r,-l),r),u=c*(c+2*l)+h):(d=Math.max(0,-(o*r+a)),c=d>0?r:Math.min(Math.max(-r,-l),r),u=-d*d+c*(c+2*l)+h);else c=o>0?-r:r,d=Math.max(0,-(o*c+a)),u=-d*d+c*(c+2*l)+h;return i&&i.copy(this.origin).addScaledVector(this.direction,d),s&&s.copy(ya).addScaledVector(wr,c),u}intersectSphere(t,e){wn.subVectors(t.center,this.origin);let i=wn.dot(this.direction),s=wn.dot(wn)-i*i,r=t.radius*t.radius;if(s>r)return null;let o=Math.sqrt(r-s),a=i-o,l=i+o;return l<0?null:a<0?this.at(l,e):this.at(a,e)}intersectsSphere(t){return this.distanceSqToPoint(t.center)<=t.radius*t.radius}distanceToPlane(t){let e=t.normal.dot(this.direction);if(e===0)return t.distanceToPoint(this.origin)===0?0:null;let i=-(this.origin.dot(t.normal)+t.constant)/e;return i>=0?i:null}intersectPlane(t,e){let i=this.distanceToPlane(t);return i===null?null:this.at(i,e)}intersectsPlane(t){let e=t.distanceToPoint(this.origin);return e===0||t.normal.dot(this.direction)*e<0}intersectBox(t,e){let i,s,r,o,a,l,h=1/this.direction.x,f=1/this.direction.y,d=1/this.direction.z,c=this.origin;return h>=0?(i=(t.min.x-c.x)*h,s=(t.max.x-c.x)*h):(i=(t.max.x-c.x)*h,s=(t.min.x-c.x)*h),f>=0?(r=(t.min.y-c.y)*f,o=(t.max.y-c.y)*f):(r=(t.max.y-c.y)*f,o=(t.min.y-c.y)*f),i>o||r>s||((r>i||isNaN(i))&&(i=r),(o<s||isNaN(s))&&(s=o),d>=0?(a=(t.min.z-c.z)*d,l=(t.max.z-c.z)*d):(a=(t.max.z-c.z)*d,l=(t.min.z-c.z)*d),i>l||a>s)||((a>i||i!==i)&&(i=a),(l<s||s!==s)&&(s=l),s<0)?null:this.at(i>=0?i:s,e)}intersectsBox(t){return this.intersectBox(t,wn)!==null}intersectTriangle(t,e,i,s,r){Ma.subVectors(e,t),Ar.subVectors(i,t),Sa.crossVectors(Ma,Ar);let o=this.direction.dot(Sa),a;if(o>0){if(s)return null;a=1}else if(o<0)a=-1,o=-o;else return null;Vn.subVectors(this.origin,t);let l=a*this.direction.dot(Ar.crossVectors(Vn,Ar));if(l<0)return null;let h=a*this.direction.dot(Ma.cross(Vn));if(h<0||l+h>o)return null;let f=-a*Vn.dot(Sa);return f<0?null:this.at(f/o,r)}applyMatrix4(t){return this.origin.applyMatrix4(t),this.direction.transformDirection(t),this}equals(t){return t.origin.equals(this.origin)&&t.direction.equals(this.direction)}clone(){return new this.constructor().copy(this)}},$t=class n{constructor(t,e,i,s,r,o,a,l,h,f,d,c,u,g,y,p){n.prototype.isMatrix4=!0,this.elements=[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1],t!==void 0&&this.set(t,e,i,s,r,o,a,l,h,f,d,c,u,g,y,p)}set(t,e,i,s,r,o,a,l,h,f,d,c,u,g,y,p){let m=this.elements;return m[0]=t,m[4]=e,m[8]=i,m[12]=s,m[1]=r,m[5]=o,m[9]=a,m[13]=l,m[2]=h,m[6]=f,m[10]=d,m[14]=c,m[3]=u,m[7]=g,m[11]=y,m[15]=p,this}identity(){return this.set(1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1),this}clone(){return new n().fromArray(this.elements)}copy(t){let e=this.elements,i=t.elements;return e[0]=i[0],e[1]=i[1],e[2]=i[2],e[3]=i[3],e[4]=i[4],e[5]=i[5],e[6]=i[6],e[7]=i[7],e[8]=i[8],e[9]=i[9],e[10]=i[10],e[11]=i[11],e[12]=i[12],e[13]=i[13],e[14]=i[14],e[15]=i[15],this}copyPosition(t){let e=this.elements,i=t.elements;return e[12]=i[12],e[13]=i[13],e[14]=i[14],this}setFromMatrix3(t){let e=t.elements;return this.set(e[0],e[3],e[6],0,e[1],e[4],e[7],0,e[2],e[5],e[8],0,0,0,0,1),this}extractBasis(t,e,i){return t.setFromMatrixColumn(this,0),e.setFromMatrixColumn(this,1),i.setFromMatrixColumn(this,2),this}makeBasis(t,e,i){return this.set(t.x,e.x,i.x,0,t.y,e.y,i.y,0,t.z,e.z,i.z,0,0,0,0,1),this}extractRotation(t){let e=this.elements,i=t.elements,s=1/Xi.setFromMatrixColumn(t,0).length(),r=1/Xi.setFromMatrixColumn(t,1).length(),o=1/Xi.setFromMatrixColumn(t,2).length();return e[0]=i[0]*s,e[1]=i[1]*s,e[2]=i[2]*s,e[3]=0,e[4]=i[4]*r,e[5]=i[5]*r,e[6]=i[6]*r,e[7]=0,e[8]=i[8]*o,e[9]=i[9]*o,e[10]=i[10]*o,e[11]=0,e[12]=0,e[13]=0,e[14]=0,e[15]=1,this}makeRotationFromEuler(t){let e=this.elements,i=t.x,s=t.y,r=t.z,o=Math.cos(i),a=Math.sin(i),l=Math.cos(s),h=Math.sin(s),f=Math.cos(r),d=Math.sin(r);if(t.order==="XYZ"){let c=o*f,u=o*d,g=a*f,y=a*d;e[0]=l*f,e[4]=-l*d,e[8]=h,e[1]=u+g*h,e[5]=c-y*h,e[9]=-a*l,e[2]=y-c*h,e[6]=g+u*h,e[10]=o*l}else if(t.order==="YXZ"){let c=l*f,u=l*d,g=h*f,y=h*d;e[0]=c+y*a,e[4]=g*a-u,e[8]=o*h,e[1]=o*d,e[5]=o*f,e[9]=-a,e[2]=u*a-g,e[6]=y+c*a,e[10]=o*l}else if(t.order==="ZXY"){let c=l*f,u=l*d,g=h*f,y=h*d;e[0]=c-y*a,e[4]=-o*d,e[8]=g+u*a,e[1]=u+g*a,e[5]=o*f,e[9]=y-c*a,e[2]=-o*h,e[6]=a,e[10]=o*l}else if(t.order==="ZYX"){let c=o*f,u=o*d,g=a*f,y=a*d;e[0]=l*f,e[4]=g*h-u,e[8]=c*h+y,e[1]=l*d,e[5]=y*h+c,e[9]=u*h-g,e[2]=-h,e[6]=a*l,e[10]=o*l}else if(t.order==="YZX"){let c=o*l,u=o*h,g=a*l,y=a*h;e[0]=l*f,e[4]=y-c*d,e[8]=g*d+u,e[1]=d,e[5]=o*f,e[9]=-a*f,e[2]=-h*f,e[6]=u*d+g,e[10]=c-y*d}else if(t.order==="XZY"){let c=o*l,u=o*h,g=a*l,y=a*h;e[0]=l*f,e[4]=-d,e[8]=h*f,e[1]=c*d+y,e[5]=o*f,e[9]=u*d-g,e[2]=g*d-u,e[6]=a*f,e[10]=y*d+c}return e[3]=0,e[7]=0,e[11]=0,e[12]=0,e[13]=0,e[14]=0,e[15]=1,this}makeRotationFromQuaternion(t){return this.compose(qf,t,Yf)}lookAt(t,e,i){let s=this.elements;return Ge.subVectors(t,e),Ge.lengthSq()===0&&(Ge.z=1),Ge.normalize(),Gn.crossVectors(i,Ge),Gn.lengthSq()===0&&(Math.abs(i.z)===1?Ge.x+=1e-4:Ge.z+=1e-4,Ge.normalize(),Gn.crossVectors(i,Ge)),Gn.normalize(),Er.crossVectors(Ge,Gn),s[0]=Gn.x,s[4]=Er.x,s[8]=Ge.x,s[1]=Gn.y,s[5]=Er.y,s[9]=Ge.y,s[2]=Gn.z,s[6]=Er.z,s[10]=Ge.z,this}multiply(t){return this.multiplyMatrices(this,t)}premultiply(t){return this.multiplyMatrices(t,this)}multiplyMatrices(t,e){let i=t.elements,s=e.elements,r=this.elements,o=i[0],a=i[4],l=i[8],h=i[12],f=i[1],d=i[5],c=i[9],u=i[13],g=i[2],y=i[6],p=i[10],m=i[14],w=i[3],_=i[7],A=i[11],I=i[15],C=s[0],E=s[4],T=s[8],H=s[12],x=s[1],S=s[5],O=s[9],W=s[13],q=s[2],et=s[6],F=s[10],it=s[14],Z=s[3],gt=s[7],_t=s[11],St=s[15];return r[0]=o*C+a*x+l*q+h*Z,r[4]=o*E+a*S+l*et+h*gt,r[8]=o*T+a*O+l*F+h*_t,r[12]=o*H+a*W+l*it+h*St,r[1]=f*C+d*x+c*q+u*Z,r[5]=f*E+d*S+c*et+u*gt,r[9]=f*T+d*O+c*F+u*_t,r[13]=f*H+d*W+c*it+u*St,r[2]=g*C+y*x+p*q+m*Z,r[6]=g*E+y*S+p*et+m*gt,r[10]=g*T+y*O+p*F+m*_t,r[14]=g*H+y*W+p*it+m*St,r[3]=w*C+_*x+A*q+I*Z,r[7]=w*E+_*S+A*et+I*gt,r[11]=w*T+_*O+A*F+I*_t,r[15]=w*H+_*W+A*it+I*St,this}multiplyScalar(t){let e=this.elements;return e[0]*=t,e[4]*=t,e[8]*=t,e[12]*=t,e[1]*=t,e[5]*=t,e[9]*=t,e[13]*=t,e[2]*=t,e[6]*=t,e[10]*=t,e[14]*=t,e[3]*=t,e[7]*=t,e[11]*=t,e[15]*=t,this}determinant(){let t=this.elements,e=t[0],i=t[4],s=t[8],r=t[12],o=t[1],a=t[5],l=t[9],h=t[13],f=t[2],d=t[6],c=t[10],u=t[14],g=t[3],y=t[7],p=t[11],m=t[15];return g*(+r*l*d-s*h*d-r*a*c+i*h*c+s*a*u-i*l*u)+y*(+e*l*u-e*h*c+r*o*c-s*o*u+s*h*f-r*l*f)+p*(+e*h*d-e*a*u-r*o*d+i*o*u+r*a*f-i*h*f)+m*(-s*a*f-e*l*d+e*a*c+s*o*d-i*o*c+i*l*f)}transpose(){let t=this.elements,e;return e=t[1],t[1]=t[4],t[4]=e,e=t[2],t[2]=t[8],t[8]=e,e=t[6],t[6]=t[9],t[9]=e,e=t[3],t[3]=t[12],t[12]=e,e=t[7],t[7]=t[13],t[13]=e,e=t[11],t[11]=t[14],t[14]=e,this}setPosition(t,e,i){let s=this.elements;return t.isVector3?(s[12]=t.x,s[13]=t.y,s[14]=t.z):(s[12]=t,s[13]=e,s[14]=i),this}invert(){let t=this.elements,e=t[0],i=t[1],s=t[2],r=t[3],o=t[4],a=t[5],l=t[6],h=t[7],f=t[8],d=t[9],c=t[10],u=t[11],g=t[12],y=t[13],p=t[14],m=t[15],w=d*p*h-y*c*h+y*l*u-a*p*u-d*l*m+a*c*m,_=g*c*h-f*p*h-g*l*u+o*p*u+f*l*m-o*c*m,A=f*y*h-g*d*h+g*a*u-o*y*u-f*a*m+o*d*m,I=g*d*l-f*y*l-g*a*c+o*y*c+f*a*p-o*d*p,C=e*w+i*_+s*A+r*I;if(C===0)return this.set(0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0);let E=1/C;return t[0]=w*E,t[1]=(y*c*r-d*p*r-y*s*u+i*p*u+d*s*m-i*c*m)*E,t[2]=(a*p*r-y*l*r+y*s*h-i*p*h-a*s*m+i*l*m)*E,t[3]=(d*l*r-a*c*r-d*s*h+i*c*h+a*s*u-i*l*u)*E,t[4]=_*E,t[5]=(f*p*r-g*c*r+g*s*u-e*p*u-f*s*m+e*c*m)*E,t[6]=(g*l*r-o*p*r-g*s*h+e*p*h+o*s*m-e*l*m)*E,t[7]=(o*c*r-f*l*r+f*s*h-e*c*h-o*s*u+e*l*u)*E,t[8]=A*E,t[9]=(g*d*r-f*y*r-g*i*u+e*y*u+f*i*m-e*d*m)*E,t[10]=(o*y*r-g*a*r+g*i*h-e*y*h-o*i*m+e*a*m)*E,t[11]=(f*a*r-o*d*r-f*i*h+e*d*h+o*i*u-e*a*u)*E,t[12]=I*E,t[13]=(f*y*s-g*d*s+g*i*c-e*y*c-f*i*p+e*d*p)*E,t[14]=(g*a*s-o*y*s-g*i*l+e*y*l+o*i*p-e*a*p)*E,t[15]=(o*d*s-f*a*s+f*i*l-e*d*l-o*i*c+e*a*c)*E,this}scale(t){let e=this.elements,i=t.x,s=t.y,r=t.z;return e[0]*=i,e[4]*=s,e[8]*=r,e[1]*=i,e[5]*=s,e[9]*=r,e[2]*=i,e[6]*=s,e[10]*=r,e[3]*=i,e[7]*=s,e[11]*=r,this}getMaxScaleOnAxis(){let t=this.elements,e=t[0]*t[0]+t[1]*t[1]+t[2]*t[2],i=t[4]*t[4]+t[5]*t[5]+t[6]*t[6],s=t[8]*t[8]+t[9]*t[9]+t[10]*t[10];return Math.sqrt(Math.max(e,i,s))}makeTranslation(t,e,i){return t.isVector3?this.set(1,0,0,t.x,0,1,0,t.y,0,0,1,t.z,0,0,0,1):this.set(1,0,0,t,0,1,0,e,0,0,1,i,0,0,0,1),this}makeRotationX(t){let e=Math.cos(t),i=Math.sin(t);return this.set(1,0,0,0,0,e,-i,0,0,i,e,0,0,0,0,1),this}makeRotationY(t){let e=Math.cos(t),i=Math.sin(t);return this.set(e,0,i,0,0,1,0,0,-i,0,e,0,0,0,0,1),this}makeRotationZ(t){let e=Math.cos(t),i=Math.sin(t);return this.set(e,-i,0,0,i,e,0,0,0,0,1,0,0,0,0,1),this}makeRotationAxis(t,e){let i=Math.cos(e),s=Math.sin(e),r=1-i,o=t.x,a=t.y,l=t.z,h=r*o,f=r*a;return this.set(h*o+i,h*a-s*l,h*l+s*a,0,h*a+s*l,f*a+i,f*l-s*o,0,h*l-s*a,f*l+s*o,r*l*l+i,0,0,0,0,1),this}makeScale(t,e,i){return this.set(t,0,0,0,0,e,0,0,0,0,i,0,0,0,0,1),this}makeShear(t,e,i,s,r,o){return this.set(1,i,r,0,t,1,o,0,e,s,1,0,0,0,0,1),this}compose(t,e,i){let s=this.elements,r=e._x,o=e._y,a=e._z,l=e._w,h=r+r,f=o+o,d=a+a,c=r*h,u=r*f,g=r*d,y=o*f,p=o*d,m=a*d,w=l*h,_=l*f,A=l*d,I=i.x,C=i.y,E=i.z;return s[0]=(1-(y+m))*I,s[1]=(u+A)*I,s[2]=(g-_)*I,s[3]=0,s[4]=(u-A)*C,s[5]=(1-(c+m))*C,s[6]=(p+w)*C,s[7]=0,s[8]=(g+_)*E,s[9]=(p-w)*E,s[10]=(1-(c+y))*E,s[11]=0,s[12]=t.x,s[13]=t.y,s[14]=t.z,s[15]=1,this}decompose(t,e,i){let s=this.elements,r=Xi.set(s[0],s[1],s[2]).length(),o=Xi.set(s[4],s[5],s[6]).length(),a=Xi.set(s[8],s[9],s[10]).length();this.determinant()<0&&(r=-r),t.x=s[12],t.y=s[13],t.z=s[14],an.copy(this);let h=1/r,f=1/o,d=1/a;return an.elements[0]*=h,an.elements[1]*=h,an.elements[2]*=h,an.elements[4]*=f,an.elements[5]*=f,an.elements[6]*=f,an.elements[8]*=d,an.elements[9]*=d,an.elements[10]*=d,e.setFromRotationMatrix(an),i.x=r,i.y=o,i.z=a,this}makePerspective(t,e,i,s,r,o,a=Pn){let l=this.elements,h=2*r/(e-t),f=2*r/(i-s),d=(e+t)/(e-t),c=(i+s)/(i-s),u,g;if(a===Pn)u=-(o+r)/(o-r),g=-2*o*r/(o-r);else if(a===$r)u=-o/(o-r),g=-o*r/(o-r);else throw new Error("THREE.Matrix4.makePerspective(): Invalid coordinate system: "+a);return l[0]=h,l[4]=0,l[8]=d,l[12]=0,l[1]=0,l[5]=f,l[9]=c,l[13]=0,l[2]=0,l[6]=0,l[10]=u,l[14]=g,l[3]=0,l[7]=0,l[11]=-1,l[15]=0,this}makeOrthographic(t,e,i,s,r,o,a=Pn){let l=this.elements,h=1/(e-t),f=1/(i-s),d=1/(o-r),c=(e+t)*h,u=(i+s)*f,g,y;if(a===Pn)g=(o+r)*d,y=-2*d;else if(a===$r)g=r*d,y=-1*d;else throw new Error("THREE.Matrix4.makeOrthographic(): Invalid coordinate system: "+a);return l[0]=2*h,l[4]=0,l[8]=0,l[12]=-c,l[1]=0,l[5]=2*f,l[9]=0,l[13]=-u,l[2]=0,l[6]=0,l[10]=y,l[14]=-g,l[3]=0,l[7]=0,l[11]=0,l[15]=1,this}equals(t){let e=this.elements,i=t.elements;for(let s=0;s<16;s++)if(e[s]!==i[s])return!1;return!0}fromArray(t,e=0){for(let i=0;i<16;i++)this.elements[i]=t[i+e];return this}toArray(t=[],e=0){let i=this.elements;return t[e]=i[0],t[e+1]=i[1],t[e+2]=i[2],t[e+3]=i[3],t[e+4]=i[4],t[e+5]=i[5],t[e+6]=i[6],t[e+7]=i[7],t[e+8]=i[8],t[e+9]=i[9],t[e+10]=i[10],t[e+11]=i[11],t[e+12]=i[12],t[e+13]=i[13],t[e+14]=i[14],t[e+15]=i[15],t}},Xi=new U,an=new $t,qf=new U(0,0,0),Yf=new U(1,1,1),Gn=new U,Er=new U,Ge=new U,_h=new $t,yh=new $e,tn=class n{constructor(t=0,e=0,i=0,s=n.DEFAULT_ORDER){this.isEuler=!0,this._x=t,this._y=e,this._z=i,this._order=s}get x(){return this._x}set x(t){this._x=t,this._onChangeCallback()}get y(){return this._y}set y(t){this._y=t,this._onChangeCallback()}get z(){return this._z}set z(t){this._z=t,this._onChangeCallback()}get order(){return this._order}set order(t){this._order=t,this._onChangeCallback()}set(t,e,i,s=this._order){return this._x=t,this._y=e,this._z=i,this._order=s,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._order)}copy(t){return this._x=t._x,this._y=t._y,this._z=t._z,this._order=t._order,this._onChangeCallback(),this}setFromRotationMatrix(t,e=this._order,i=!0){let s=t.elements,r=s[0],o=s[4],a=s[8],l=s[1],h=s[5],f=s[9],d=s[2],c=s[6],u=s[10];switch(e){case"XYZ":this._y=Math.asin(Le(a,-1,1)),Math.abs(a)<.9999999?(this._x=Math.atan2(-f,u),this._z=Math.atan2(-o,r)):(this._x=Math.atan2(c,h),this._z=0);break;case"YXZ":this._x=Math.asin(-Le(f,-1,1)),Math.abs(f)<.9999999?(this._y=Math.atan2(a,u),this._z=Math.atan2(l,h)):(this._y=Math.atan2(-d,r),this._z=0);break;case"ZXY":this._x=Math.asin(Le(c,-1,1)),Math.abs(c)<.9999999?(this._y=Math.atan2(-d,u),this._z=Math.atan2(-o,h)):(this._y=0,this._z=Math.atan2(l,r));break;case"ZYX":this._y=Math.asin(-Le(d,-1,1)),Math.abs(d)<.9999999?(this._x=Math.atan2(c,u),this._z=Math.atan2(l,r)):(this._x=0,this._z=Math.atan2(-o,h));break;case"YZX":this._z=Math.asin(Le(l,-1,1)),Math.abs(l)<.9999999?(this._x=Math.atan2(-f,h),this._y=Math.atan2(-d,r)):(this._x=0,this._y=Math.atan2(a,u));break;case"XZY":this._z=Math.asin(-Le(o,-1,1)),Math.abs(o)<.9999999?(this._x=Math.atan2(c,h),this._y=Math.atan2(a,r)):(this._x=Math.atan2(-f,u),this._y=0);break;default:console.warn("THREE.Euler: .setFromRotationMatrix() encountered an unknown order: "+e)}return this._order=e,i===!0&&this._onChangeCallback(),this}setFromQuaternion(t,e,i){return _h.makeRotationFromQuaternion(t),this.setFromRotationMatrix(_h,e,i)}setFromVector3(t,e=this._order){return this.set(t.x,t.y,t.z,e)}reorder(t){return yh.setFromEuler(this),this.setFromQuaternion(yh,t)}equals(t){return t._x===this._x&&t._y===this._y&&t._z===this._z&&t._order===this._order}fromArray(t){return this._x=t[0],this._y=t[1],this._z=t[2],t[3]!==void 0&&(this._order=t[3]),this._onChangeCallback(),this}toArray(t=[],e=0){return t[e]=this._x,t[e+1]=this._y,t[e+2]=this._z,t[e+3]=this._order,t}_onChange(t){return this._onChangeCallback=t,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._order}};tn.DEFAULT_ORDER="XYZ";var tr=class{constructor(){this.mask=1}set(t){this.mask=(1<<t|0)>>>0}enable(t){this.mask|=1<<t|0}enableAll(){this.mask=-1}toggle(t){this.mask^=1<<t|0}disable(t){this.mask&=~(1<<t|0)}disableAll(){this.mask=0}test(t){return(this.mask&t.mask)!==0}isEnabled(t){return(this.mask&(1<<t|0))!==0}},Zf=0,Mh=new U,qi=new $e,An=new $t,Tr=new U,Ws=new U,jf=new U,Kf=new $e,Sh=new U(1,0,0),bh=new U(0,1,0),wh=new U(0,0,1),Ah={type:"added"},Jf={type:"removed"},Yi={type:"childadded",child:null},ba={type:"childremoved",child:null},Be=class n extends Ln{constructor(){super(),this.isObject3D=!0,Object.defineProperty(this,"id",{value:Zf++}),this.uuid=xs(),this.name="",this.type="Object3D",this.parent=null,this.children=[],this.up=n.DEFAULT_UP.clone();let t=new U,e=new tn,i=new $e,s=new U(1,1,1);function r(){i.setFromEuler(e,!1)}function o(){e.setFromQuaternion(i,void 0,!1)}e._onChange(r),i._onChange(o),Object.defineProperties(this,{position:{configurable:!0,enumerable:!0,value:t},rotation:{configurable:!0,enumerable:!0,value:e},quaternion:{configurable:!0,enumerable:!0,value:i},scale:{configurable:!0,enumerable:!0,value:s},modelViewMatrix:{value:new $t},normalMatrix:{value:new zt}}),this.matrix=new $t,this.matrixWorld=new $t,this.matrixAutoUpdate=n.DEFAULT_MATRIX_AUTO_UPDATE,this.matrixWorldAutoUpdate=n.DEFAULT_MATRIX_WORLD_AUTO_UPDATE,this.matrixWorldNeedsUpdate=!1,this.layers=new tr,this.visible=!0,this.castShadow=!1,this.receiveShadow=!1,this.frustumCulled=!0,this.renderOrder=0,this.animations=[],this.userData={}}onBeforeShadow(){}onAfterShadow(){}onBeforeRender(){}onAfterRender(){}applyMatrix4(t){this.matrixAutoUpdate&&this.updateMatrix(),this.matrix.premultiply(t),this.matrix.decompose(this.position,this.quaternion,this.scale)}applyQuaternion(t){return this.quaternion.premultiply(t),this}setRotationFromAxisAngle(t,e){this.quaternion.setFromAxisAngle(t,e)}setRotationFromEuler(t){this.quaternion.setFromEuler(t,!0)}setRotationFromMatrix(t){this.quaternion.setFromRotationMatrix(t)}setRotationFromQuaternion(t){this.quaternion.copy(t)}rotateOnAxis(t,e){return qi.setFromAxisAngle(t,e),this.quaternion.multiply(qi),this}rotateOnWorldAxis(t,e){return qi.setFromAxisAngle(t,e),this.quaternion.premultiply(qi),this}rotateX(t){return this.rotateOnAxis(Sh,t)}rotateY(t){return this.rotateOnAxis(bh,t)}rotateZ(t){return this.rotateOnAxis(wh,t)}translateOnAxis(t,e){return Mh.copy(t).applyQuaternion(this.quaternion),this.position.add(Mh.multiplyScalar(e)),this}translateX(t){return this.translateOnAxis(Sh,t)}translateY(t){return this.translateOnAxis(bh,t)}translateZ(t){return this.translateOnAxis(wh,t)}localToWorld(t){return this.updateWorldMatrix(!0,!1),t.applyMatrix4(this.matrixWorld)}worldToLocal(t){return this.updateWorldMatrix(!0,!1),t.applyMatrix4(An.copy(this.matrixWorld).invert())}lookAt(t,e,i){t.isVector3?Tr.copy(t):Tr.set(t,e,i);let s=this.parent;this.updateWorldMatrix(!0,!1),Ws.setFromMatrixPosition(this.matrixWorld),this.isCamera||this.isLight?An.lookAt(Ws,Tr,this.up):An.lookAt(Tr,Ws,this.up),this.quaternion.setFromRotationMatrix(An),s&&(An.extractRotation(s.matrixWorld),qi.setFromRotationMatrix(An),this.quaternion.premultiply(qi.invert()))}add(t){if(arguments.length>1){for(let e=0;e<arguments.length;e++)this.add(arguments[e]);return this}return t===this?(console.error("THREE.Object3D.add: object can't be added as a child of itself.",t),this):(t&&t.isObject3D?(t.removeFromParent(),t.parent=this,this.children.push(t),t.dispatchEvent(Ah),Yi.child=t,this.dispatchEvent(Yi),Yi.child=null):console.error("THREE.Object3D.add: object not an instance of THREE.Object3D.",t),this)}remove(t){if(arguments.length>1){for(let i=0;i<arguments.length;i++)this.remove(arguments[i]);return this}let e=this.children.indexOf(t);return e!==-1&&(t.parent=null,this.children.splice(e,1),t.dispatchEvent(Jf),ba.child=t,this.dispatchEvent(ba),ba.child=null),this}removeFromParent(){let t=this.parent;return t!==null&&t.remove(this),this}clear(){return this.remove(...this.children)}attach(t){return this.updateWorldMatrix(!0,!1),An.copy(this.matrixWorld).invert(),t.parent!==null&&(t.parent.updateWorldMatrix(!0,!1),An.multiply(t.parent.matrixWorld)),t.applyMatrix4(An),t.removeFromParent(),t.parent=this,this.children.push(t),t.updateWorldMatrix(!1,!0),t.dispatchEvent(Ah),Yi.child=t,this.dispatchEvent(Yi),Yi.child=null,this}getObjectById(t){return this.getObjectByProperty("id",t)}getObjectByName(t){return this.getObjectByProperty("name",t)}getObjectByProperty(t,e){if(this[t]===e)return this;for(let i=0,s=this.children.length;i<s;i++){let o=this.children[i].getObjectByProperty(t,e);if(o!==void 0)return o}}getObjectsByProperty(t,e,i=[]){this[t]===e&&i.push(this);let s=this.children;for(let r=0,o=s.length;r<o;r++)s[r].getObjectsByProperty(t,e,i);return i}getWorldPosition(t){return this.updateWorldMatrix(!0,!1),t.setFromMatrixPosition(this.matrixWorld)}getWorldQuaternion(t){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(Ws,t,jf),t}getWorldScale(t){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(Ws,Kf,t),t}getWorldDirection(t){this.updateWorldMatrix(!0,!1);let e=this.matrixWorld.elements;return t.set(e[8],e[9],e[10]).normalize()}raycast(){}traverse(t){t(this);let e=this.children;for(let i=0,s=e.length;i<s;i++)e[i].traverse(t)}traverseVisible(t){if(this.visible===!1)return;t(this);let e=this.children;for(let i=0,s=e.length;i<s;i++)e[i].traverseVisible(t)}traverseAncestors(t){let e=this.parent;e!==null&&(t(e),e.traverseAncestors(t))}updateMatrix(){this.matrix.compose(this.position,this.quaternion,this.scale),this.matrixWorldNeedsUpdate=!0}updateMatrixWorld(t){this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||t)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,t=!0);let e=this.children;for(let i=0,s=e.length;i<s;i++)e[i].updateMatrixWorld(t)}updateWorldMatrix(t,e){let i=this.parent;if(t===!0&&i!==null&&i.updateWorldMatrix(!0,!1),this.matrixAutoUpdate&&this.updateMatrix(),this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),e===!0){let s=this.children;for(let r=0,o=s.length;r<o;r++)s[r].updateWorldMatrix(!1,!0)}}toJSON(t){let e=t===void 0||typeof t=="string",i={};e&&(t={geometries:{},materials:{},textures:{},images:{},shapes:{},skeletons:{},animations:{},nodes:{}},i.metadata={version:4.6,type:"Object",generator:"Object3D.toJSON"});let s={};s.uuid=this.uuid,s.type=this.type,this.name!==""&&(s.name=this.name),this.castShadow===!0&&(s.castShadow=!0),this.receiveShadow===!0&&(s.receiveShadow=!0),this.visible===!1&&(s.visible=!1),this.frustumCulled===!1&&(s.frustumCulled=!1),this.renderOrder!==0&&(s.renderOrder=this.renderOrder),Object.keys(this.userData).length>0&&(s.userData=this.userData),s.layers=this.layers.mask,s.matrix=this.matrix.toArray(),s.up=this.up.toArray(),this.matrixAutoUpdate===!1&&(s.matrixAutoUpdate=!1),this.isInstancedMesh&&(s.type="InstancedMesh",s.count=this.count,s.instanceMatrix=this.instanceMatrix.toJSON(),this.instanceColor!==null&&(s.instanceColor=this.instanceColor.toJSON())),this.isBatchedMesh&&(s.type="BatchedMesh",s.perObjectFrustumCulled=this.perObjectFrustumCulled,s.sortObjects=this.sortObjects,s.drawRanges=this._drawRanges,s.reservedRanges=this._reservedRanges,s.visibility=this._visibility,s.active=this._active,s.bounds=this._bounds.map(a=>({boxInitialized:a.boxInitialized,boxMin:a.box.min.toArray(),boxMax:a.box.max.toArray(),sphereInitialized:a.sphereInitialized,sphereRadius:a.sphere.radius,sphereCenter:a.sphere.center.toArray()})),s.maxInstanceCount=this._maxInstanceCount,s.maxVertexCount=this._maxVertexCount,s.maxIndexCount=this._maxIndexCount,s.geometryInitialized=this._geometryInitialized,s.geometryCount=this._geometryCount,s.matricesTexture=this._matricesTexture.toJSON(t),this._colorsTexture!==null&&(s.colorsTexture=this._colorsTexture.toJSON(t)),this.boundingSphere!==null&&(s.boundingSphere={center:s.boundingSphere.center.toArray(),radius:s.boundingSphere.radius}),this.boundingBox!==null&&(s.boundingBox={min:s.boundingBox.min.toArray(),max:s.boundingBox.max.toArray()}));function r(a,l){return a[l.uuid]===void 0&&(a[l.uuid]=l.toJSON(t)),l.uuid}if(this.isScene)this.background&&(this.background.isColor?s.background=this.background.toJSON():this.background.isTexture&&(s.background=this.background.toJSON(t).uuid)),this.environment&&this.environment.isTexture&&this.environment.isRenderTargetTexture!==!0&&(s.environment=this.environment.toJSON(t).uuid);else if(this.isMesh||this.isLine||this.isPoints){s.geometry=r(t.geometries,this.geometry);let a=this.geometry.parameters;if(a!==void 0&&a.shapes!==void 0){let l=a.shapes;if(Array.isArray(l))for(let h=0,f=l.length;h<f;h++){let d=l[h];r(t.shapes,d)}else r(t.shapes,l)}}if(this.isSkinnedMesh&&(s.bindMode=this.bindMode,s.bindMatrix=this.bindMatrix.toArray(),this.skeleton!==void 0&&(r(t.skeletons,this.skeleton),s.skeleton=this.skeleton.uuid)),this.material!==void 0)if(Array.isArray(this.material)){let a=[];for(let l=0,h=this.material.length;l<h;l++)a.push(r(t.materials,this.material[l]));s.material=a}else s.material=r(t.materials,this.material);if(this.children.length>0){s.children=[];for(let a=0;a<this.children.length;a++)s.children.push(this.children[a].toJSON(t).object)}if(this.animations.length>0){s.animations=[];for(let a=0;a<this.animations.length;a++){let l=this.animations[a];s.animations.push(r(t.animations,l))}}if(e){let a=o(t.geometries),l=o(t.materials),h=o(t.textures),f=o(t.images),d=o(t.shapes),c=o(t.skeletons),u=o(t.animations),g=o(t.nodes);a.length>0&&(i.geometries=a),l.length>0&&(i.materials=l),h.length>0&&(i.textures=h),f.length>0&&(i.images=f),d.length>0&&(i.shapes=d),c.length>0&&(i.skeletons=c),u.length>0&&(i.animations=u),g.length>0&&(i.nodes=g)}return i.object=s,i;function o(a){let l=[];for(let h in a){let f=a[h];delete f.metadata,l.push(f)}return l}}clone(t){return new this.constructor().copy(this,t)}copy(t,e=!0){if(this.name=t.name,this.up.copy(t.up),this.position.copy(t.position),this.rotation.order=t.rotation.order,this.quaternion.copy(t.quaternion),this.scale.copy(t.scale),this.matrix.copy(t.matrix),this.matrixWorld.copy(t.matrixWorld),this.matrixAutoUpdate=t.matrixAutoUpdate,this.matrixWorldAutoUpdate=t.matrixWorldAutoUpdate,this.matrixWorldNeedsUpdate=t.matrixWorldNeedsUpdate,this.layers.mask=t.layers.mask,this.visible=t.visible,this.castShadow=t.castShadow,this.receiveShadow=t.receiveShadow,this.frustumCulled=t.frustumCulled,this.renderOrder=t.renderOrder,this.animations=t.animations.slice(),this.userData=JSON.parse(JSON.stringify(t.userData)),e===!0)for(let i=0;i<t.children.length;i++){let s=t.children[i];this.add(s.clone())}return this}};Be.DEFAULT_UP=new U(0,1,0);Be.DEFAULT_MATRIX_AUTO_UPDATE=!0;Be.DEFAULT_MATRIX_WORLD_AUTO_UPDATE=!0;var ln=new U,En=new U,wa=new U,Tn=new U,Zi=new U,ji=new U,Eh=new U,Aa=new U,Ea=new U,Ta=new U,Ca=new oe,Ra=new oe,Pa=new oe,pi=class n{constructor(t=new U,e=new U,i=new U){this.a=t,this.b=e,this.c=i}static getNormal(t,e,i,s){s.subVectors(i,e),ln.subVectors(t,e),s.cross(ln);let r=s.lengthSq();return r>0?s.multiplyScalar(1/Math.sqrt(r)):s.set(0,0,0)}static getBarycoord(t,e,i,s,r){ln.subVectors(s,e),En.subVectors(i,e),wa.subVectors(t,e);let o=ln.dot(ln),a=ln.dot(En),l=ln.dot(wa),h=En.dot(En),f=En.dot(wa),d=o*h-a*a;if(d===0)return r.set(0,0,0),null;let c=1/d,u=(h*l-a*f)*c,g=(o*f-a*l)*c;return r.set(1-u-g,g,u)}static containsPoint(t,e,i,s){return this.getBarycoord(t,e,i,s,Tn)===null?!1:Tn.x>=0&&Tn.y>=0&&Tn.x+Tn.y<=1}static getInterpolation(t,e,i,s,r,o,a,l){return this.getBarycoord(t,e,i,s,Tn)===null?(l.x=0,l.y=0,"z"in l&&(l.z=0),"w"in l&&(l.w=0),null):(l.setScalar(0),l.addScaledVector(r,Tn.x),l.addScaledVector(o,Tn.y),l.addScaledVector(a,Tn.z),l)}static getInterpolatedAttribute(t,e,i,s,r,o){return Ca.setScalar(0),Ra.setScalar(0),Pa.setScalar(0),Ca.fromBufferAttribute(t,e),Ra.fromBufferAttribute(t,i),Pa.fromBufferAttribute(t,s),o.setScalar(0),o.addScaledVector(Ca,r.x),o.addScaledVector(Ra,r.y),o.addScaledVector(Pa,r.z),o}static isFrontFacing(t,e,i,s){return ln.subVectors(i,e),En.subVectors(t,e),ln.cross(En).dot(s)<0}set(t,e,i){return this.a.copy(t),this.b.copy(e),this.c.copy(i),this}setFromPointsAndIndices(t,e,i,s){return this.a.copy(t[e]),this.b.copy(t[i]),this.c.copy(t[s]),this}setFromAttributeAndIndices(t,e,i,s){return this.a.fromBufferAttribute(t,e),this.b.fromBufferAttribute(t,i),this.c.fromBufferAttribute(t,s),this}clone(){return new this.constructor().copy(this)}copy(t){return this.a.copy(t.a),this.b.copy(t.b),this.c.copy(t.c),this}getArea(){return ln.subVectors(this.c,this.b),En.subVectors(this.a,this.b),ln.cross(En).length()*.5}getMidpoint(t){return t.addVectors(this.a,this.b).add(this.c).multiplyScalar(1/3)}getNormal(t){return n.getNormal(this.a,this.b,this.c,t)}getPlane(t){return t.setFromCoplanarPoints(this.a,this.b,this.c)}getBarycoord(t,e){return n.getBarycoord(t,this.a,this.b,this.c,e)}getInterpolation(t,e,i,s,r){return n.getInterpolation(t,this.a,this.b,this.c,e,i,s,r)}containsPoint(t){return n.containsPoint(t,this.a,this.b,this.c)}isFrontFacing(t){return n.isFrontFacing(this.a,this.b,this.c,t)}intersectsBox(t){return t.intersectsTriangle(this)}closestPointToPoint(t,e){let i=this.a,s=this.b,r=this.c,o,a;Zi.subVectors(s,i),ji.subVectors(r,i),Aa.subVectors(t,i);let l=Zi.dot(Aa),h=ji.dot(Aa);if(l<=0&&h<=0)return e.copy(i);Ea.subVectors(t,s);let f=Zi.dot(Ea),d=ji.dot(Ea);if(f>=0&&d<=f)return e.copy(s);let c=l*d-f*h;if(c<=0&&l>=0&&f<=0)return o=l/(l-f),e.copy(i).addScaledVector(Zi,o);Ta.subVectors(t,r);let u=Zi.dot(Ta),g=ji.dot(Ta);if(g>=0&&u<=g)return e.copy(r);let y=u*h-l*g;if(y<=0&&h>=0&&g<=0)return a=h/(h-g),e.copy(i).addScaledVector(ji,a);let p=f*g-u*d;if(p<=0&&d-f>=0&&u-g>=0)return Eh.subVectors(r,s),a=(d-f)/(d-f+(u-g)),e.copy(s).addScaledVector(Eh,a);let m=1/(p+y+c);return o=y*m,a=c*m,e.copy(i).addScaledVector(Zi,o).addScaledVector(ji,a)}equals(t){return t.a.equals(this.a)&&t.b.equals(this.b)&&t.c.equals(this.c)}},bu={aliceblue:15792383,antiquewhite:16444375,aqua:65535,aquamarine:8388564,azure:15794175,beige:16119260,bisque:16770244,black:0,blanchedalmond:16772045,blue:255,blueviolet:9055202,brown:10824234,burlywood:14596231,cadetblue:6266528,chartreuse:8388352,chocolate:13789470,coral:16744272,cornflowerblue:6591981,cornsilk:16775388,crimson:14423100,cyan:65535,darkblue:139,darkcyan:35723,darkgoldenrod:12092939,darkgray:11119017,darkgreen:25600,darkgrey:11119017,darkkhaki:12433259,darkmagenta:9109643,darkolivegreen:5597999,darkorange:16747520,darkorchid:10040012,darkred:9109504,darksalmon:15308410,darkseagreen:9419919,darkslateblue:4734347,darkslategray:3100495,darkslategrey:3100495,darkturquoise:52945,darkviolet:9699539,deeppink:16716947,deepskyblue:49151,dimgray:6908265,dimgrey:6908265,dodgerblue:2003199,firebrick:11674146,floralwhite:16775920,forestgreen:2263842,fuchsia:16711935,gainsboro:14474460,ghostwhite:16316671,gold:16766720,goldenrod:14329120,gray:8421504,green:32768,greenyellow:11403055,grey:8421504,honeydew:15794160,hotpink:16738740,indianred:13458524,indigo:4915330,ivory:16777200,khaki:15787660,lavender:15132410,lavenderblush:16773365,lawngreen:8190976,lemonchiffon:16775885,lightblue:11393254,lightcoral:15761536,lightcyan:14745599,lightgoldenrodyellow:16448210,lightgray:13882323,lightgreen:9498256,lightgrey:13882323,lightpink:16758465,lightsalmon:16752762,lightseagreen:2142890,lightskyblue:8900346,lightslategray:7833753,lightslategrey:7833753,lightsteelblue:11584734,lightyellow:16777184,lime:65280,limegreen:3329330,linen:16445670,magenta:16711935,maroon:8388608,mediumaquamarine:6737322,mediumblue:205,mediumorchid:12211667,mediumpurple:9662683,mediumseagreen:3978097,mediumslateblue:8087790,mediumspringgreen:64154,mediumturquoise:4772300,mediumvioletred:13047173,midnightblue:1644912,mintcream:16121850,mistyrose:16770273,moccasin:16770229,navajowhite:16768685,navy:128,oldlace:16643558,olive:8421376,olivedrab:7048739,orange:16753920,orangered:16729344,orchid:14315734,palegoldenrod:15657130,palegreen:10025880,paleturquoise:11529966,palevioletred:14381203,papayawhip:16773077,peachpuff:16767673,peru:13468991,pink:16761035,plum:14524637,powderblue:11591910,purple:8388736,rebeccapurple:6697881,red:16711680,rosybrown:12357519,royalblue:4286945,saddlebrown:9127187,salmon:16416882,sandybrown:16032864,seagreen:3050327,seashell:16774638,sienna:10506797,silver:12632256,skyblue:8900331,slateblue:6970061,slategray:7372944,slategrey:7372944,snow:16775930,springgreen:65407,steelblue:4620980,tan:13808780,teal:32896,thistle:14204888,tomato:16737095,turquoise:4251856,violet:15631086,wheat:16113331,white:16777215,whitesmoke:16119285,yellow:16776960,yellowgreen:10145074},Wn={h:0,s:0,l:0},Cr={h:0,s:0,l:0};function Ia(n,t,e){return e<0&&(e+=1),e>1&&(e-=1),e<1/6?n+(t-n)*6*e:e<1/2?t:e<2/3?n+(t-n)*6*(2/3-e):n}var Ot=class{constructor(t,e,i){return this.isColor=!0,this.r=1,this.g=1,this.b=1,this.set(t,e,i)}set(t,e,i){if(e===void 0&&i===void 0){let s=t;s&&s.isColor?this.copy(s):typeof s=="number"?this.setHex(s):typeof s=="string"&&this.setStyle(s)}else this.setRGB(t,e,i);return this}setScalar(t){return this.r=t,this.g=t,this.b=t,this}setHex(t,e=mn){return t=Math.floor(t),this.r=(t>>16&255)/255,this.g=(t>>8&255)/255,this.b=(t&255)/255,Jt.toWorkingColorSpace(this,e),this}setRGB(t,e,i,s=Jt.workingColorSpace){return this.r=t,this.g=e,this.b=i,Jt.toWorkingColorSpace(this,s),this}setHSL(t,e,i,s=Jt.workingColorSpace){if(t=fc(t,1),e=Le(e,0,1),i=Le(i,0,1),e===0)this.r=this.g=this.b=i;else{let r=i<=.5?i*(1+e):i+e-i*e,o=2*i-r;this.r=Ia(o,r,t+1/3),this.g=Ia(o,r,t),this.b=Ia(o,r,t-1/3)}return Jt.toWorkingColorSpace(this,s),this}setStyle(t,e=mn){function i(r){r!==void 0&&parseFloat(r)<1&&console.warn("THREE.Color: Alpha component of "+t+" will be ignored.")}let s;if(s=/^(\w+)\(([^\)]*)\)/.exec(t)){let r,o=s[1],a=s[2];switch(o){case"rgb":case"rgba":if(r=/^\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return i(r[4]),this.setRGB(Math.min(255,parseInt(r[1],10))/255,Math.min(255,parseInt(r[2],10))/255,Math.min(255,parseInt(r[3],10))/255,e);if(r=/^\s*(\d+)\%\s*,\s*(\d+)\%\s*,\s*(\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return i(r[4]),this.setRGB(Math.min(100,parseInt(r[1],10))/100,Math.min(100,parseInt(r[2],10))/100,Math.min(100,parseInt(r[3],10))/100,e);break;case"hsl":case"hsla":if(r=/^\s*(\d*\.?\d+)\s*,\s*(\d*\.?\d+)\%\s*,\s*(\d*\.?\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return i(r[4]),this.setHSL(parseFloat(r[1])/360,parseFloat(r[2])/100,parseFloat(r[3])/100,e);break;default:console.warn("THREE.Color: Unknown color model "+t)}}else if(s=/^\#([A-Fa-f\d]+)$/.exec(t)){let r=s[1],o=r.length;if(o===3)return this.setRGB(parseInt(r.charAt(0),16)/15,parseInt(r.charAt(1),16)/15,parseInt(r.charAt(2),16)/15,e);if(o===6)return this.setHex(parseInt(r,16),e);console.warn("THREE.Color: Invalid hex color "+t)}else if(t&&t.length>0)return this.setColorName(t,e);return this}setColorName(t,e=mn){let i=bu[t.toLowerCase()];return i!==void 0?this.setHex(i,e):console.warn("THREE.Color: Unknown color "+t),this}clone(){return new this.constructor(this.r,this.g,this.b)}copy(t){return this.r=t.r,this.g=t.g,this.b=t.b,this}copySRGBToLinear(t){return this.r=rs(t.r),this.g=rs(t.g),this.b=rs(t.b),this}copyLinearToSRGB(t){return this.r=ma(t.r),this.g=ma(t.g),this.b=ma(t.b),this}convertSRGBToLinear(){return this.copySRGBToLinear(this),this}convertLinearToSRGB(){return this.copyLinearToSRGB(this),this}getHex(t=mn){return Jt.fromWorkingColorSpace(Te.copy(this),t),Math.round(Le(Te.r*255,0,255))*65536+Math.round(Le(Te.g*255,0,255))*256+Math.round(Le(Te.b*255,0,255))}getHexString(t=mn){return("000000"+this.getHex(t).toString(16)).slice(-6)}getHSL(t,e=Jt.workingColorSpace){Jt.fromWorkingColorSpace(Te.copy(this),e);let i=Te.r,s=Te.g,r=Te.b,o=Math.max(i,s,r),a=Math.min(i,s,r),l,h,f=(a+o)/2;if(a===o)l=0,h=0;else{let d=o-a;switch(h=f<=.5?d/(o+a):d/(2-o-a),o){case i:l=(s-r)/d+(s<r?6:0);break;case s:l=(r-i)/d+2;break;case r:l=(i-s)/d+4;break}l/=6}return t.h=l,t.s=h,t.l=f,t}getRGB(t,e=Jt.workingColorSpace){return Jt.fromWorkingColorSpace(Te.copy(this),e),t.r=Te.r,t.g=Te.g,t.b=Te.b,t}getStyle(t=mn){Jt.fromWorkingColorSpace(Te.copy(this),t);let e=Te.r,i=Te.g,s=Te.b;return t!==mn?`color(${t} ${e.toFixed(3)} ${i.toFixed(3)} ${s.toFixed(3)})`:`rgb(${Math.round(e*255)},${Math.round(i*255)},${Math.round(s*255)})`}offsetHSL(t,e,i){return this.getHSL(Wn),this.setHSL(Wn.h+t,Wn.s+e,Wn.l+i)}add(t){return this.r+=t.r,this.g+=t.g,this.b+=t.b,this}addColors(t,e){return this.r=t.r+e.r,this.g=t.g+e.g,this.b=t.b+e.b,this}addScalar(t){return this.r+=t,this.g+=t,this.b+=t,this}sub(t){return this.r=Math.max(0,this.r-t.r),this.g=Math.max(0,this.g-t.g),this.b=Math.max(0,this.b-t.b),this}multiply(t){return this.r*=t.r,this.g*=t.g,this.b*=t.b,this}multiplyScalar(t){return this.r*=t,this.g*=t,this.b*=t,this}lerp(t,e){return this.r+=(t.r-this.r)*e,this.g+=(t.g-this.g)*e,this.b+=(t.b-this.b)*e,this}lerpColors(t,e,i){return this.r=t.r+(e.r-t.r)*i,this.g=t.g+(e.g-t.g)*i,this.b=t.b+(e.b-t.b)*i,this}lerpHSL(t,e){this.getHSL(Wn),t.getHSL(Cr);let i=Ks(Wn.h,Cr.h,e),s=Ks(Wn.s,Cr.s,e),r=Ks(Wn.l,Cr.l,e);return this.setHSL(i,s,r),this}setFromVector3(t){return this.r=t.x,this.g=t.y,this.b=t.z,this}applyMatrix3(t){let e=this.r,i=this.g,s=this.b,r=t.elements;return this.r=r[0]*e+r[3]*i+r[6]*s,this.g=r[1]*e+r[4]*i+r[7]*s,this.b=r[2]*e+r[5]*i+r[8]*s,this}equals(t){return t.r===this.r&&t.g===this.g&&t.b===this.b}fromArray(t,e=0){return this.r=t[e],this.g=t[e+1],this.b=t[e+2],this}toArray(t=[],e=0){return t[e]=this.r,t[e+1]=this.g,t[e+2]=this.b,t}fromBufferAttribute(t,e){return this.r=t.getX(e),this.g=t.getY(e),this.b=t.getZ(e),this}toJSON(){return this.getHex()}*[Symbol.iterator](){yield this.r,yield this.g,yield this.b}},Te=new Ot;Ot.NAMES=bu;var Qf=0,yi=class extends Ln{constructor(){super(),this.isMaterial=!0,Object.defineProperty(this,"id",{value:Qf++}),this.uuid=xs(),this.name="",this.type="Material",this.blending=is,this.side=jn,this.vertexColors=!1,this.opacity=1,this.transparent=!1,this.alphaHash=!1,this.blendSrc=Va,this.blendDst=Ga,this.blendEquation=fi,this.blendSrcAlpha=null,this.blendDstAlpha=null,this.blendEquationAlpha=null,this.blendColor=new Ot(0,0,0),this.blendAlpha=0,this.depthFunc=ls,this.depthTest=!0,this.depthWrite=!0,this.stencilWriteMask=255,this.stencilFunc=uh,this.stencilRef=0,this.stencilFuncMask=255,this.stencilFail=ki,this.stencilZFail=ki,this.stencilZPass=ki,this.stencilWrite=!1,this.clippingPlanes=null,this.clipIntersection=!1,this.clipShadows=!1,this.shadowSide=null,this.colorWrite=!0,this.precision=null,this.polygonOffset=!1,this.polygonOffsetFactor=0,this.polygonOffsetUnits=0,this.dithering=!1,this.alphaToCoverage=!1,this.premultipliedAlpha=!1,this.forceSinglePass=!1,this.visible=!0,this.toneMapped=!0,this.userData={},this.version=0,this._alphaTest=0}get alphaTest(){return this._alphaTest}set alphaTest(t){this._alphaTest>0!=t>0&&this.version++,this._alphaTest=t}onBeforeRender(){}onBeforeCompile(){}customProgramCacheKey(){return this.onBeforeCompile.toString()}setValues(t){if(t!==void 0)for(let e in t){let i=t[e];if(i===void 0){console.warn(`THREE.Material: parameter '${e}' has value of undefined.`);continue}let s=this[e];if(s===void 0){console.warn(`THREE.Material: '${e}' is not a property of THREE.${this.type}.`);continue}s&&s.isColor?s.set(i):s&&s.isVector3&&i&&i.isVector3?s.copy(i):this[e]=i}}toJSON(t){let e=t===void 0||typeof t=="string";e&&(t={textures:{},images:{}});let i={metadata:{version:4.6,type:"Material",generator:"Material.toJSON"}};i.uuid=this.uuid,i.type=this.type,this.name!==""&&(i.name=this.name),this.color&&this.color.isColor&&(i.color=this.color.getHex()),this.roughness!==void 0&&(i.roughness=this.roughness),this.metalness!==void 0&&(i.metalness=this.metalness),this.sheen!==void 0&&(i.sheen=this.sheen),this.sheenColor&&this.sheenColor.isColor&&(i.sheenColor=this.sheenColor.getHex()),this.sheenRoughness!==void 0&&(i.sheenRoughness=this.sheenRoughness),this.emissive&&this.emissive.isColor&&(i.emissive=this.emissive.getHex()),this.emissiveIntensity!==void 0&&this.emissiveIntensity!==1&&(i.emissiveIntensity=this.emissiveIntensity),this.specular&&this.specular.isColor&&(i.specular=this.specular.getHex()),this.specularIntensity!==void 0&&(i.specularIntensity=this.specularIntensity),this.specularColor&&this.specularColor.isColor&&(i.specularColor=this.specularColor.getHex()),this.shininess!==void 0&&(i.shininess=this.shininess),this.clearcoat!==void 0&&(i.clearcoat=this.clearcoat),this.clearcoatRoughness!==void 0&&(i.clearcoatRoughness=this.clearcoatRoughness),this.clearcoatMap&&this.clearcoatMap.isTexture&&(i.clearcoatMap=this.clearcoatMap.toJSON(t).uuid),this.clearcoatRoughnessMap&&this.clearcoatRoughnessMap.isTexture&&(i.clearcoatRoughnessMap=this.clearcoatRoughnessMap.toJSON(t).uuid),this.clearcoatNormalMap&&this.clearcoatNormalMap.isTexture&&(i.clearcoatNormalMap=this.clearcoatNormalMap.toJSON(t).uuid,i.clearcoatNormalScale=this.clearcoatNormalScale.toArray()),this.dispersion!==void 0&&(i.dispersion=this.dispersion),this.iridescence!==void 0&&(i.iridescence=this.iridescence),this.iridescenceIOR!==void 0&&(i.iridescenceIOR=this.iridescenceIOR),this.iridescenceThicknessRange!==void 0&&(i.iridescenceThicknessRange=this.iridescenceThicknessRange),this.iridescenceMap&&this.iridescenceMap.isTexture&&(i.iridescenceMap=this.iridescenceMap.toJSON(t).uuid),this.iridescenceThicknessMap&&this.iridescenceThicknessMap.isTexture&&(i.iridescenceThicknessMap=this.iridescenceThicknessMap.toJSON(t).uuid),this.anisotropy!==void 0&&(i.anisotropy=this.anisotropy),this.anisotropyRotation!==void 0&&(i.anisotropyRotation=this.anisotropyRotation),this.anisotropyMap&&this.anisotropyMap.isTexture&&(i.anisotropyMap=this.anisotropyMap.toJSON(t).uuid),this.map&&this.map.isTexture&&(i.map=this.map.toJSON(t).uuid),this.matcap&&this.matcap.isTexture&&(i.matcap=this.matcap.toJSON(t).uuid),this.alphaMap&&this.alphaMap.isTexture&&(i.alphaMap=this.alphaMap.toJSON(t).uuid),this.lightMap&&this.lightMap.isTexture&&(i.lightMap=this.lightMap.toJSON(t).uuid,i.lightMapIntensity=this.lightMapIntensity),this.aoMap&&this.aoMap.isTexture&&(i.aoMap=this.aoMap.toJSON(t).uuid,i.aoMapIntensity=this.aoMapIntensity),this.bumpMap&&this.bumpMap.isTexture&&(i.bumpMap=this.bumpMap.toJSON(t).uuid,i.bumpScale=this.bumpScale),this.normalMap&&this.normalMap.isTexture&&(i.normalMap=this.normalMap.toJSON(t).uuid,i.normalMapType=this.normalMapType,i.normalScale=this.normalScale.toArray()),this.displacementMap&&this.displacementMap.isTexture&&(i.displacementMap=this.displacementMap.toJSON(t).uuid,i.displacementScale=this.displacementScale,i.displacementBias=this.displacementBias),this.roughnessMap&&this.roughnessMap.isTexture&&(i.roughnessMap=this.roughnessMap.toJSON(t).uuid),this.metalnessMap&&this.metalnessMap.isTexture&&(i.metalnessMap=this.metalnessMap.toJSON(t).uuid),this.emissiveMap&&this.emissiveMap.isTexture&&(i.emissiveMap=this.emissiveMap.toJSON(t).uuid),this.specularMap&&this.specularMap.isTexture&&(i.specularMap=this.specularMap.toJSON(t).uuid),this.specularIntensityMap&&this.specularIntensityMap.isTexture&&(i.specularIntensityMap=this.specularIntensityMap.toJSON(t).uuid),this.specularColorMap&&this.specularColorMap.isTexture&&(i.specularColorMap=this.specularColorMap.toJSON(t).uuid),this.envMap&&this.envMap.isTexture&&(i.envMap=this.envMap.toJSON(t).uuid,this.combine!==void 0&&(i.combine=this.combine)),this.envMapRotation!==void 0&&(i.envMapRotation=this.envMapRotation.toArray()),this.envMapIntensity!==void 0&&(i.envMapIntensity=this.envMapIntensity),this.reflectivity!==void 0&&(i.reflectivity=this.reflectivity),this.refractionRatio!==void 0&&(i.refractionRatio=this.refractionRatio),this.gradientMap&&this.gradientMap.isTexture&&(i.gradientMap=this.gradientMap.toJSON(t).uuid),this.transmission!==void 0&&(i.transmission=this.transmission),this.transmissionMap&&this.transmissionMap.isTexture&&(i.transmissionMap=this.transmissionMap.toJSON(t).uuid),this.thickness!==void 0&&(i.thickness=this.thickness),this.thicknessMap&&this.thicknessMap.isTexture&&(i.thicknessMap=this.thicknessMap.toJSON(t).uuid),this.attenuationDistance!==void 0&&this.attenuationDistance!==1/0&&(i.attenuationDistance=this.attenuationDistance),this.attenuationColor!==void 0&&(i.attenuationColor=this.attenuationColor.getHex()),this.size!==void 0&&(i.size=this.size),this.shadowSide!==null&&(i.shadowSide=this.shadowSide),this.sizeAttenuation!==void 0&&(i.sizeAttenuation=this.sizeAttenuation),this.blending!==is&&(i.blending=this.blending),this.side!==jn&&(i.side=this.side),this.vertexColors===!0&&(i.vertexColors=!0),this.opacity<1&&(i.opacity=this.opacity),this.transparent===!0&&(i.transparent=!0),this.blendSrc!==Va&&(i.blendSrc=this.blendSrc),this.blendDst!==Ga&&(i.blendDst=this.blendDst),this.blendEquation!==fi&&(i.blendEquation=this.blendEquation),this.blendSrcAlpha!==null&&(i.blendSrcAlpha=this.blendSrcAlpha),this.blendDstAlpha!==null&&(i.blendDstAlpha=this.blendDstAlpha),this.blendEquationAlpha!==null&&(i.blendEquationAlpha=this.blendEquationAlpha),this.blendColor&&this.blendColor.isColor&&(i.blendColor=this.blendColor.getHex()),this.blendAlpha!==0&&(i.blendAlpha=this.blendAlpha),this.depthFunc!==ls&&(i.depthFunc=this.depthFunc),this.depthTest===!1&&(i.depthTest=this.depthTest),this.depthWrite===!1&&(i.depthWrite=this.depthWrite),this.colorWrite===!1&&(i.colorWrite=this.colorWrite),this.stencilWriteMask!==255&&(i.stencilWriteMask=this.stencilWriteMask),this.stencilFunc!==uh&&(i.stencilFunc=this.stencilFunc),this.stencilRef!==0&&(i.stencilRef=this.stencilRef),this.stencilFuncMask!==255&&(i.stencilFuncMask=this.stencilFuncMask),this.stencilFail!==ki&&(i.stencilFail=this.stencilFail),this.stencilZFail!==ki&&(i.stencilZFail=this.stencilZFail),this.stencilZPass!==ki&&(i.stencilZPass=this.stencilZPass),this.stencilWrite===!0&&(i.stencilWrite=this.stencilWrite),this.rotation!==void 0&&this.rotation!==0&&(i.rotation=this.rotation),this.polygonOffset===!0&&(i.polygonOffset=!0),this.polygonOffsetFactor!==0&&(i.polygonOffsetFactor=this.polygonOffsetFactor),this.polygonOffsetUnits!==0&&(i.polygonOffsetUnits=this.polygonOffsetUnits),this.linewidth!==void 0&&this.linewidth!==1&&(i.linewidth=this.linewidth),this.dashSize!==void 0&&(i.dashSize=this.dashSize),this.gapSize!==void 0&&(i.gapSize=this.gapSize),this.scale!==void 0&&(i.scale=this.scale),this.dithering===!0&&(i.dithering=!0),this.alphaTest>0&&(i.alphaTest=this.alphaTest),this.alphaHash===!0&&(i.alphaHash=!0),this.alphaToCoverage===!0&&(i.alphaToCoverage=!0),this.premultipliedAlpha===!0&&(i.premultipliedAlpha=!0),this.forceSinglePass===!0&&(i.forceSinglePass=!0),this.wireframe===!0&&(i.wireframe=!0),this.wireframeLinewidth>1&&(i.wireframeLinewidth=this.wireframeLinewidth),this.wireframeLinecap!=="round"&&(i.wireframeLinecap=this.wireframeLinecap),this.wireframeLinejoin!=="round"&&(i.wireframeLinejoin=this.wireframeLinejoin),this.flatShading===!0&&(i.flatShading=!0),this.visible===!1&&(i.visible=!1),this.toneMapped===!1&&(i.toneMapped=!1),this.fog===!1&&(i.fog=!1),Object.keys(this.userData).length>0&&(i.userData=this.userData);function s(r){let o=[];for(let a in r){let l=r[a];delete l.metadata,o.push(l)}return o}if(e){let r=s(t.textures),o=s(t.images);r.length>0&&(i.textures=r),o.length>0&&(i.images=o)}return i}clone(){return new this.constructor().copy(this)}copy(t){this.name=t.name,this.blending=t.blending,this.side=t.side,this.vertexColors=t.vertexColors,this.opacity=t.opacity,this.transparent=t.transparent,this.blendSrc=t.blendSrc,this.blendDst=t.blendDst,this.blendEquation=t.blendEquation,this.blendSrcAlpha=t.blendSrcAlpha,this.blendDstAlpha=t.blendDstAlpha,this.blendEquationAlpha=t.blendEquationAlpha,this.blendColor.copy(t.blendColor),this.blendAlpha=t.blendAlpha,this.depthFunc=t.depthFunc,this.depthTest=t.depthTest,this.depthWrite=t.depthWrite,this.stencilWriteMask=t.stencilWriteMask,this.stencilFunc=t.stencilFunc,this.stencilRef=t.stencilRef,this.stencilFuncMask=t.stencilFuncMask,this.stencilFail=t.stencilFail,this.stencilZFail=t.stencilZFail,this.stencilZPass=t.stencilZPass,this.stencilWrite=t.stencilWrite;let e=t.clippingPlanes,i=null;if(e!==null){let s=e.length;i=new Array(s);for(let r=0;r!==s;++r)i[r]=e[r].clone()}return this.clippingPlanes=i,this.clipIntersection=t.clipIntersection,this.clipShadows=t.clipShadows,this.shadowSide=t.shadowSide,this.colorWrite=t.colorWrite,this.precision=t.precision,this.polygonOffset=t.polygonOffset,this.polygonOffsetFactor=t.polygonOffsetFactor,this.polygonOffsetUnits=t.polygonOffsetUnits,this.dithering=t.dithering,this.alphaTest=t.alphaTest,this.alphaHash=t.alphaHash,this.alphaToCoverage=t.alphaToCoverage,this.premultipliedAlpha=t.premultipliedAlpha,this.forceSinglePass=t.forceSinglePass,this.visible=t.visible,this.toneMapped=t.toneMapped,this.userData=JSON.parse(JSON.stringify(t.userData)),this}dispose(){this.dispatchEvent({type:"dispose"})}set needsUpdate(t){t===!0&&this.version++}onBuild(){console.warn("Material: onBuild() has been removed.")}},fs=class extends yi{constructor(t){super(),this.isMeshBasicMaterial=!0,this.type="MeshBasicMaterial",this.color=new Ot(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new tn,this.combine=cu,this.reflectivity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.fog=!0,this.setValues(t)}copy(t){return super.copy(t),this.color.copy(t.color),this.map=t.map,this.lightMap=t.lightMap,this.lightMapIntensity=t.lightMapIntensity,this.aoMap=t.aoMap,this.aoMapIntensity=t.aoMapIntensity,this.specularMap=t.specularMap,this.alphaMap=t.alphaMap,this.envMap=t.envMap,this.envMapRotation.copy(t.envMapRotation),this.combine=t.combine,this.reflectivity=t.reflectivity,this.refractionRatio=t.refractionRatio,this.wireframe=t.wireframe,this.wireframeLinewidth=t.wireframeLinewidth,this.wireframeLinecap=t.wireframeLinecap,this.wireframeLinejoin=t.wireframeLinejoin,this.fog=t.fog,this}};var he=new U,Rr=new yt,qe=class{constructor(t,e,i=!1){if(Array.isArray(t))throw new TypeError("THREE.BufferAttribute: array should be a Typed Array.");this.isBufferAttribute=!0,this.name="",this.array=t,this.itemSize=e,this.count=t!==void 0?t.length/e:0,this.normalized=i,this.usage=dh,this.updateRanges=[],this.gpuType=xn,this.version=0}onUploadCallback(){}set needsUpdate(t){t===!0&&this.version++}setUsage(t){return this.usage=t,this}addUpdateRange(t,e){this.updateRanges.push({start:t,count:e})}clearUpdateRanges(){this.updateRanges.length=0}copy(t){return this.name=t.name,this.array=new t.array.constructor(t.array),this.itemSize=t.itemSize,this.count=t.count,this.normalized=t.normalized,this.usage=t.usage,this.gpuType=t.gpuType,this}copyAt(t,e,i){t*=this.itemSize,i*=e.itemSize;for(let s=0,r=this.itemSize;s<r;s++)this.array[t+s]=e.array[i+s];return this}copyArray(t){return this.array.set(t),this}applyMatrix3(t){if(this.itemSize===2)for(let e=0,i=this.count;e<i;e++)Rr.fromBufferAttribute(this,e),Rr.applyMatrix3(t),this.setXY(e,Rr.x,Rr.y);else if(this.itemSize===3)for(let e=0,i=this.count;e<i;e++)he.fromBufferAttribute(this,e),he.applyMatrix3(t),this.setXYZ(e,he.x,he.y,he.z);return this}applyMatrix4(t){for(let e=0,i=this.count;e<i;e++)he.fromBufferAttribute(this,e),he.applyMatrix4(t),this.setXYZ(e,he.x,he.y,he.z);return this}applyNormalMatrix(t){for(let e=0,i=this.count;e<i;e++)he.fromBufferAttribute(this,e),he.applyNormalMatrix(t),this.setXYZ(e,he.x,he.y,he.z);return this}transformDirection(t){for(let e=0,i=this.count;e<i;e++)he.fromBufferAttribute(this,e),he.transformDirection(t),this.setXYZ(e,he.x,he.y,he.z);return this}set(t,e=0){return this.array.set(t,e),this}getComponent(t,e){let i=this.array[t*this.itemSize+e];return this.normalized&&(i=es(i,this.array)),i}setComponent(t,e,i){return this.normalized&&(i=Pe(i,this.array)),this.array[t*this.itemSize+e]=i,this}getX(t){let e=this.array[t*this.itemSize];return this.normalized&&(e=es(e,this.array)),e}setX(t,e){return this.normalized&&(e=Pe(e,this.array)),this.array[t*this.itemSize]=e,this}getY(t){let e=this.array[t*this.itemSize+1];return this.normalized&&(e=es(e,this.array)),e}setY(t,e){return this.normalized&&(e=Pe(e,this.array)),this.array[t*this.itemSize+1]=e,this}getZ(t){let e=this.array[t*this.itemSize+2];return this.normalized&&(e=es(e,this.array)),e}setZ(t,e){return this.normalized&&(e=Pe(e,this.array)),this.array[t*this.itemSize+2]=e,this}getW(t){let e=this.array[t*this.itemSize+3];return this.normalized&&(e=es(e,this.array)),e}setW(t,e){return this.normalized&&(e=Pe(e,this.array)),this.array[t*this.itemSize+3]=e,this}setXY(t,e,i){return t*=this.itemSize,this.normalized&&(e=Pe(e,this.array),i=Pe(i,this.array)),this.array[t+0]=e,this.array[t+1]=i,this}setXYZ(t,e,i,s){return t*=this.itemSize,this.normalized&&(e=Pe(e,this.array),i=Pe(i,this.array),s=Pe(s,this.array)),this.array[t+0]=e,this.array[t+1]=i,this.array[t+2]=s,this}setXYZW(t,e,i,s,r){return t*=this.itemSize,this.normalized&&(e=Pe(e,this.array),i=Pe(i,this.array),s=Pe(s,this.array),r=Pe(r,this.array)),this.array[t+0]=e,this.array[t+1]=i,this.array[t+2]=s,this.array[t+3]=r,this}onUpload(t){return this.onUploadCallback=t,this}clone(){return new this.constructor(this.array,this.itemSize).copy(this)}toJSON(){let t={itemSize:this.itemSize,type:this.array.constructor.name,array:Array.from(this.array),normalized:this.normalized};return this.name!==""&&(t.name=this.name),this.usage!==dh&&(t.usage=this.usage),t}};var so=class extends qe{constructor(t,e,i){super(new Uint16Array(t),e,i)}};var ro=class extends qe{constructor(t,e,i){super(new Uint32Array(t),e,i)}};var ve=class extends qe{constructor(t,e,i){super(new Float32Array(t),e,i)}},$f=0,Qe=new $t,La=new Be,Ki=new U,We=new Dn,Xs=new Dn,xe=new U,un=class n extends Ln{constructor(){super(),this.isBufferGeometry=!0,Object.defineProperty(this,"id",{value:$f++}),this.uuid=xs(),this.name="",this.type="BufferGeometry",this.index=null,this.attributes={},this.morphAttributes={},this.morphTargetsRelative=!1,this.groups=[],this.boundingBox=null,this.boundingSphere=null,this.drawRange={start:0,count:1/0},this.userData={}}getIndex(){return this.index}setIndex(t){return Array.isArray(t)?this.index=new(Su(t)?ro:so)(t,1):this.index=t,this}getAttribute(t){return this.attributes[t]}setAttribute(t,e){return this.attributes[t]=e,this}deleteAttribute(t){return delete this.attributes[t],this}hasAttribute(t){return this.attributes[t]!==void 0}addGroup(t,e,i=0){this.groups.push({start:t,count:e,materialIndex:i})}clearGroups(){this.groups=[]}setDrawRange(t,e){this.drawRange.start=t,this.drawRange.count=e}applyMatrix4(t){let e=this.attributes.position;e!==void 0&&(e.applyMatrix4(t),e.needsUpdate=!0);let i=this.attributes.normal;if(i!==void 0){let r=new zt().getNormalMatrix(t);i.applyNormalMatrix(r),i.needsUpdate=!0}let s=this.attributes.tangent;return s!==void 0&&(s.transformDirection(t),s.needsUpdate=!0),this.boundingBox!==null&&this.computeBoundingBox(),this.boundingSphere!==null&&this.computeBoundingSphere(),this}applyQuaternion(t){return Qe.makeRotationFromQuaternion(t),this.applyMatrix4(Qe),this}rotateX(t){return Qe.makeRotationX(t),this.applyMatrix4(Qe),this}rotateY(t){return Qe.makeRotationY(t),this.applyMatrix4(Qe),this}rotateZ(t){return Qe.makeRotationZ(t),this.applyMatrix4(Qe),this}translate(t,e,i){return Qe.makeTranslation(t,e,i),this.applyMatrix4(Qe),this}scale(t,e,i){return Qe.makeScale(t,e,i),this.applyMatrix4(Qe),this}lookAt(t){return La.lookAt(t),La.updateMatrix(),this.applyMatrix4(La.matrix),this}center(){return this.computeBoundingBox(),this.boundingBox.getCenter(Ki).negate(),this.translate(Ki.x,Ki.y,Ki.z),this}setFromPoints(t){let e=[];for(let i=0,s=t.length;i<s;i++){let r=t[i];e.push(r.x,r.y,r.z||0)}return this.setAttribute("position",new ve(e,3)),this}computeBoundingBox(){this.boundingBox===null&&(this.boundingBox=new Dn);let t=this.attributes.position,e=this.morphAttributes.position;if(t&&t.isGLBufferAttribute){console.error("THREE.BufferGeometry.computeBoundingBox(): GLBufferAttribute requires a manual bounding box.",this),this.boundingBox.set(new U(-1/0,-1/0,-1/0),new U(1/0,1/0,1/0));return}if(t!==void 0){if(this.boundingBox.setFromBufferAttribute(t),e)for(let i=0,s=e.length;i<s;i++){let r=e[i];We.setFromBufferAttribute(r),this.morphTargetsRelative?(xe.addVectors(this.boundingBox.min,We.min),this.boundingBox.expandByPoint(xe),xe.addVectors(this.boundingBox.max,We.max),this.boundingBox.expandByPoint(xe)):(this.boundingBox.expandByPoint(We.min),this.boundingBox.expandByPoint(We.max))}}else this.boundingBox.makeEmpty();(isNaN(this.boundingBox.min.x)||isNaN(this.boundingBox.min.y)||isNaN(this.boundingBox.min.z))&&console.error('THREE.BufferGeometry.computeBoundingBox(): Computed min/max have NaN values. The "position" attribute is likely to have NaN values.',this)}computeBoundingSphere(){this.boundingSphere===null&&(this.boundingSphere=new _i);let t=this.attributes.position,e=this.morphAttributes.position;if(t&&t.isGLBufferAttribute){console.error("THREE.BufferGeometry.computeBoundingSphere(): GLBufferAttribute requires a manual bounding sphere.",this),this.boundingSphere.set(new U,1/0);return}if(t){let i=this.boundingSphere.center;if(We.setFromBufferAttribute(t),e)for(let r=0,o=e.length;r<o;r++){let a=e[r];Xs.setFromBufferAttribute(a),this.morphTargetsRelative?(xe.addVectors(We.min,Xs.min),We.expandByPoint(xe),xe.addVectors(We.max,Xs.max),We.expandByPoint(xe)):(We.expandByPoint(Xs.min),We.expandByPoint(Xs.max))}We.getCenter(i);let s=0;for(let r=0,o=t.count;r<o;r++)xe.fromBufferAttribute(t,r),s=Math.max(s,i.distanceToSquared(xe));if(e)for(let r=0,o=e.length;r<o;r++){let a=e[r],l=this.morphTargetsRelative;for(let h=0,f=a.count;h<f;h++)xe.fromBufferAttribute(a,h),l&&(Ki.fromBufferAttribute(t,h),xe.add(Ki)),s=Math.max(s,i.distanceToSquared(xe))}this.boundingSphere.radius=Math.sqrt(s),isNaN(this.boundingSphere.radius)&&console.error('THREE.BufferGeometry.computeBoundingSphere(): Computed radius is NaN. The "position" attribute is likely to have NaN values.',this)}}computeTangents(){let t=this.index,e=this.attributes;if(t===null||e.position===void 0||e.normal===void 0||e.uv===void 0){console.error("THREE.BufferGeometry: .computeTangents() failed. Missing required attributes (index, position, normal or uv)");return}let i=e.position,s=e.normal,r=e.uv;this.hasAttribute("tangent")===!1&&this.setAttribute("tangent",new qe(new Float32Array(4*i.count),4));let o=this.getAttribute("tangent"),a=[],l=[];for(let T=0;T<i.count;T++)a[T]=new U,l[T]=new U;let h=new U,f=new U,d=new U,c=new yt,u=new yt,g=new yt,y=new U,p=new U;function m(T,H,x){h.fromBufferAttribute(i,T),f.fromBufferAttribute(i,H),d.fromBufferAttribute(i,x),c.fromBufferAttribute(r,T),u.fromBufferAttribute(r,H),g.fromBufferAttribute(r,x),f.sub(h),d.sub(h),u.sub(c),g.sub(c);let S=1/(u.x*g.y-g.x*u.y);isFinite(S)&&(y.copy(f).multiplyScalar(g.y).addScaledVector(d,-u.y).multiplyScalar(S),p.copy(d).multiplyScalar(u.x).addScaledVector(f,-g.x).multiplyScalar(S),a[T].add(y),a[H].add(y),a[x].add(y),l[T].add(p),l[H].add(p),l[x].add(p))}let w=this.groups;w.length===0&&(w=[{start:0,count:t.count}]);for(let T=0,H=w.length;T<H;++T){let x=w[T],S=x.start,O=x.count;for(let W=S,q=S+O;W<q;W+=3)m(t.getX(W+0),t.getX(W+1),t.getX(W+2))}let _=new U,A=new U,I=new U,C=new U;function E(T){I.fromBufferAttribute(s,T),C.copy(I);let H=a[T];_.copy(H),_.sub(I.multiplyScalar(I.dot(H))).normalize(),A.crossVectors(C,H);let S=A.dot(l[T])<0?-1:1;o.setXYZW(T,_.x,_.y,_.z,S)}for(let T=0,H=w.length;T<H;++T){let x=w[T],S=x.start,O=x.count;for(let W=S,q=S+O;W<q;W+=3)E(t.getX(W+0)),E(t.getX(W+1)),E(t.getX(W+2))}}computeVertexNormals(){let t=this.index,e=this.getAttribute("position");if(e!==void 0){let i=this.getAttribute("normal");if(i===void 0)i=new qe(new Float32Array(e.count*3),3),this.setAttribute("normal",i);else for(let c=0,u=i.count;c<u;c++)i.setXYZ(c,0,0,0);let s=new U,r=new U,o=new U,a=new U,l=new U,h=new U,f=new U,d=new U;if(t)for(let c=0,u=t.count;c<u;c+=3){let g=t.getX(c+0),y=t.getX(c+1),p=t.getX(c+2);s.fromBufferAttribute(e,g),r.fromBufferAttribute(e,y),o.fromBufferAttribute(e,p),f.subVectors(o,r),d.subVectors(s,r),f.cross(d),a.fromBufferAttribute(i,g),l.fromBufferAttribute(i,y),h.fromBufferAttribute(i,p),a.add(f),l.add(f),h.add(f),i.setXYZ(g,a.x,a.y,a.z),i.setXYZ(y,l.x,l.y,l.z),i.setXYZ(p,h.x,h.y,h.z)}else for(let c=0,u=e.count;c<u;c+=3)s.fromBufferAttribute(e,c+0),r.fromBufferAttribute(e,c+1),o.fromBufferAttribute(e,c+2),f.subVectors(o,r),d.subVectors(s,r),f.cross(d),i.setXYZ(c+0,f.x,f.y,f.z),i.setXYZ(c+1,f.x,f.y,f.z),i.setXYZ(c+2,f.x,f.y,f.z);this.normalizeNormals(),i.needsUpdate=!0}}normalizeNormals(){let t=this.attributes.normal;for(let e=0,i=t.count;e<i;e++)xe.fromBufferAttribute(t,e),xe.normalize(),t.setXYZ(e,xe.x,xe.y,xe.z)}toNonIndexed(){function t(a,l){let h=a.array,f=a.itemSize,d=a.normalized,c=new h.constructor(l.length*f),u=0,g=0;for(let y=0,p=l.length;y<p;y++){a.isInterleavedBufferAttribute?u=l[y]*a.data.stride+a.offset:u=l[y]*f;for(let m=0;m<f;m++)c[g++]=h[u++]}return new qe(c,f,d)}if(this.index===null)return console.warn("THREE.BufferGeometry.toNonIndexed(): BufferGeometry is already non-indexed."),this;let e=new n,i=this.index.array,s=this.attributes;for(let a in s){let l=s[a],h=t(l,i);e.setAttribute(a,h)}let r=this.morphAttributes;for(let a in r){let l=[],h=r[a];for(let f=0,d=h.length;f<d;f++){let c=h[f],u=t(c,i);l.push(u)}e.morphAttributes[a]=l}e.morphTargetsRelative=this.morphTargetsRelative;let o=this.groups;for(let a=0,l=o.length;a<l;a++){let h=o[a];e.addGroup(h.start,h.count,h.materialIndex)}return e}toJSON(){let t={metadata:{version:4.6,type:"BufferGeometry",generator:"BufferGeometry.toJSON"}};if(t.uuid=this.uuid,t.type=this.type,this.name!==""&&(t.name=this.name),Object.keys(this.userData).length>0&&(t.userData=this.userData),this.parameters!==void 0){let l=this.parameters;for(let h in l)l[h]!==void 0&&(t[h]=l[h]);return t}t.data={attributes:{}};let e=this.index;e!==null&&(t.data.index={type:e.array.constructor.name,array:Array.prototype.slice.call(e.array)});let i=this.attributes;for(let l in i){let h=i[l];t.data.attributes[l]=h.toJSON(t.data)}let s={},r=!1;for(let l in this.morphAttributes){let h=this.morphAttributes[l],f=[];for(let d=0,c=h.length;d<c;d++){let u=h[d];f.push(u.toJSON(t.data))}f.length>0&&(s[l]=f,r=!0)}r&&(t.data.morphAttributes=s,t.data.morphTargetsRelative=this.morphTargetsRelative);let o=this.groups;o.length>0&&(t.data.groups=JSON.parse(JSON.stringify(o)));let a=this.boundingSphere;return a!==null&&(t.data.boundingSphere={center:a.center.toArray(),radius:a.radius}),t}clone(){return new this.constructor().copy(this)}copy(t){this.index=null,this.attributes={},this.morphAttributes={},this.groups=[],this.boundingBox=null,this.boundingSphere=null;let e={};this.name=t.name;let i=t.index;i!==null&&this.setIndex(i.clone(e));let s=t.attributes;for(let h in s){let f=s[h];this.setAttribute(h,f.clone(e))}let r=t.morphAttributes;for(let h in r){let f=[],d=r[h];for(let c=0,u=d.length;c<u;c++)f.push(d[c].clone(e));this.morphAttributes[h]=f}this.morphTargetsRelative=t.morphTargetsRelative;let o=t.groups;for(let h=0,f=o.length;h<f;h++){let d=o[h];this.addGroup(d.start,d.count,d.materialIndex)}let a=t.boundingBox;a!==null&&(this.boundingBox=a.clone());let l=t.boundingSphere;return l!==null&&(this.boundingSphere=l.clone()),this.drawRange.start=t.drawRange.start,this.drawRange.count=t.drawRange.count,this.userData=t.userData,this}dispose(){this.dispatchEvent({type:"dispose"})}},Th=new $t,li=new io,Pr=new _i,Ch=new U,Ir=new U,Lr=new U,Dr=new U,Da=new U,Ur=new U,Rh=new U,Nr=new U,Ue=class extends Be{constructor(t=new un,e=new fs){super(),this.isMesh=!0,this.type="Mesh",this.geometry=t,this.material=e,this.updateMorphTargets()}copy(t,e){return super.copy(t,e),t.morphTargetInfluences!==void 0&&(this.morphTargetInfluences=t.morphTargetInfluences.slice()),t.morphTargetDictionary!==void 0&&(this.morphTargetDictionary=Object.assign({},t.morphTargetDictionary)),this.material=Array.isArray(t.material)?t.material.slice():t.material,this.geometry=t.geometry,this}updateMorphTargets(){let e=this.geometry.morphAttributes,i=Object.keys(e);if(i.length>0){let s=e[i[0]];if(s!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=s.length;r<o;r++){let a=s[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}getVertexPosition(t,e){let i=this.geometry,s=i.attributes.position,r=i.morphAttributes.position,o=i.morphTargetsRelative;e.fromBufferAttribute(s,t);let a=this.morphTargetInfluences;if(r&&a){Ur.set(0,0,0);for(let l=0,h=r.length;l<h;l++){let f=a[l],d=r[l];f!==0&&(Da.fromBufferAttribute(d,t),o?Ur.addScaledVector(Da,f):Ur.addScaledVector(Da.sub(e),f))}e.add(Ur)}return e}raycast(t,e){let i=this.geometry,s=this.material,r=this.matrixWorld;s!==void 0&&(i.boundingSphere===null&&i.computeBoundingSphere(),Pr.copy(i.boundingSphere),Pr.applyMatrix4(r),li.copy(t.ray).recast(t.near),!(Pr.containsPoint(li.origin)===!1&&(li.intersectSphere(Pr,Ch)===null||li.origin.distanceToSquared(Ch)>(t.far-t.near)**2))&&(Th.copy(r).invert(),li.copy(t.ray).applyMatrix4(Th),!(i.boundingBox!==null&&li.intersectsBox(i.boundingBox)===!1)&&this._computeIntersections(t,e,li)))}_computeIntersections(t,e,i){let s,r=this.geometry,o=this.material,a=r.index,l=r.attributes.position,h=r.attributes.uv,f=r.attributes.uv1,d=r.attributes.normal,c=r.groups,u=r.drawRange;if(a!==null)if(Array.isArray(o))for(let g=0,y=c.length;g<y;g++){let p=c[g],m=o[p.materialIndex],w=Math.max(p.start,u.start),_=Math.min(a.count,Math.min(p.start+p.count,u.start+u.count));for(let A=w,I=_;A<I;A+=3){let C=a.getX(A),E=a.getX(A+1),T=a.getX(A+2);s=Or(this,m,t,i,h,f,d,C,E,T),s&&(s.faceIndex=Math.floor(A/3),s.face.materialIndex=p.materialIndex,e.push(s))}}else{let g=Math.max(0,u.start),y=Math.min(a.count,u.start+u.count);for(let p=g,m=y;p<m;p+=3){let w=a.getX(p),_=a.getX(p+1),A=a.getX(p+2);s=Or(this,o,t,i,h,f,d,w,_,A),s&&(s.faceIndex=Math.floor(p/3),e.push(s))}}else if(l!==void 0)if(Array.isArray(o))for(let g=0,y=c.length;g<y;g++){let p=c[g],m=o[p.materialIndex],w=Math.max(p.start,u.start),_=Math.min(l.count,Math.min(p.start+p.count,u.start+u.count));for(let A=w,I=_;A<I;A+=3){let C=A,E=A+1,T=A+2;s=Or(this,m,t,i,h,f,d,C,E,T),s&&(s.faceIndex=Math.floor(A/3),s.face.materialIndex=p.materialIndex,e.push(s))}}else{let g=Math.max(0,u.start),y=Math.min(l.count,u.start+u.count);for(let p=g,m=y;p<m;p+=3){let w=p,_=p+1,A=p+2;s=Or(this,o,t,i,h,f,d,w,_,A),s&&(s.faceIndex=Math.floor(p/3),e.push(s))}}}};function tp(n,t,e,i,s,r,o,a){let l;if(t.side===Fe?l=i.intersectTriangle(o,r,s,!0,a):l=i.intersectTriangle(s,r,o,t.side===jn,a),l===null)return null;Nr.copy(a),Nr.applyMatrix4(n.matrixWorld);let h=e.ray.origin.distanceTo(Nr);return h<e.near||h>e.far?null:{distance:h,point:Nr.clone(),object:n}}function Or(n,t,e,i,s,r,o,a,l,h){n.getVertexPosition(a,Ir),n.getVertexPosition(l,Lr),n.getVertexPosition(h,Dr);let f=tp(n,t,e,i,Ir,Lr,Dr,Rh);if(f){let d=new U;pi.getBarycoord(Rh,Ir,Lr,Dr,d),s&&(f.uv=pi.getInterpolatedAttribute(s,a,l,h,d,new yt)),r&&(f.uv1=pi.getInterpolatedAttribute(r,a,l,h,d,new yt)),o&&(f.normal=pi.getInterpolatedAttribute(o,a,l,h,d,new U),f.normal.dot(i.direction)>0&&f.normal.multiplyScalar(-1));let c={a,b:l,c:h,normal:new U,materialIndex:0};pi.getNormal(Ir,Lr,Dr,c.normal),f.face=c,f.barycoord=d}return f}var er=class n extends un{constructor(t=1,e=1,i=1,s=1,r=1,o=1){super(),this.type="BoxGeometry",this.parameters={width:t,height:e,depth:i,widthSegments:s,heightSegments:r,depthSegments:o};let a=this;s=Math.floor(s),r=Math.floor(r),o=Math.floor(o);let l=[],h=[],f=[],d=[],c=0,u=0;g("z","y","x",-1,-1,i,e,t,o,r,0),g("z","y","x",1,-1,i,e,-t,o,r,1),g("x","z","y",1,1,t,i,e,s,o,2),g("x","z","y",1,-1,t,i,-e,s,o,3),g("x","y","z",1,-1,t,e,i,s,r,4),g("x","y","z",-1,-1,t,e,-i,s,r,5),this.setIndex(l),this.setAttribute("position",new ve(h,3)),this.setAttribute("normal",new ve(f,3)),this.setAttribute("uv",new ve(d,2));function g(y,p,m,w,_,A,I,C,E,T,H){let x=A/E,S=I/T,O=A/2,W=I/2,q=C/2,et=E+1,F=T+1,it=0,Z=0,gt=new U;for(let _t=0;_t<F;_t++){let St=_t*S-W;for(let Gt=0;Gt<et;Gt++){let Wt=Gt*x-O;gt[y]=Wt*w,gt[p]=St*_,gt[m]=q,h.push(gt.x,gt.y,gt.z),gt[y]=0,gt[p]=0,gt[m]=C>0?1:-1,f.push(gt.x,gt.y,gt.z),d.push(Gt/E),d.push(1-_t/T),it+=1}}for(let _t=0;_t<T;_t++)for(let St=0;St<E;St++){let Gt=c+St+et*_t,Wt=c+St+et*(_t+1),J=c+(St+1)+et*(_t+1),st=c+(St+1)+et*_t;l.push(Gt,Wt,st),l.push(Wt,J,st),Z+=6}a.addGroup(u,Z,H),u+=Z,c+=it}}copy(t){return super.copy(t),this.parameters=Object.assign({},t.parameters),this}static fromJSON(t){return new n(t.width,t.height,t.depth,t.widthSegments,t.heightSegments,t.depthSegments)}};function ps(n){let t={};for(let e in n){t[e]={};for(let i in n[e]){let s=n[e][i];s&&(s.isColor||s.isMatrix3||s.isMatrix4||s.isVector2||s.isVector3||s.isVector4||s.isTexture||s.isQuaternion)?s.isRenderTargetTexture?(console.warn("UniformsUtils: Textures of render targets cannot be cloned via cloneUniforms() or mergeUniforms()."),t[e][i]=null):t[e][i]=s.clone():Array.isArray(s)?t[e][i]=s.slice():t[e][i]=s}}return t}function Ie(n){let t={};for(let e=0;e<n.length;e++){let i=ps(n[e]);for(let s in i)t[s]=i[s]}return t}function ep(n){let t=[];for(let e=0;e<n.length;e++)t.push(n[e].clone());return t}function wu(n){let t=n.getRenderTarget();return t===null?n.outputColorSpace:t.isXRRenderTarget===!0?t.texture.colorSpace:Jt.workingColorSpace}var _n={clone:ps,merge:Ie},np=`void main() {
	gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
}`,ip=`void main() {
	gl_FragColor = vec4( 1.0, 0.0, 0.0, 1.0 );
}`,ie=class extends yi{constructor(t){super(),this.isShaderMaterial=!0,this.type="ShaderMaterial",this.defines={},this.uniforms={},this.uniformsGroups=[],this.vertexShader=np,this.fragmentShader=ip,this.linewidth=1,this.wireframe=!1,this.wireframeLinewidth=1,this.fog=!1,this.lights=!1,this.clipping=!1,this.forceSinglePass=!0,this.extensions={clipCullDistance:!1,multiDraw:!1},this.defaultAttributeValues={color:[1,1,1],uv:[0,0],uv1:[0,0]},this.index0AttributeName=void 0,this.uniformsNeedUpdate=!1,this.glslVersion=null,t!==void 0&&this.setValues(t)}copy(t){return super.copy(t),this.fragmentShader=t.fragmentShader,this.vertexShader=t.vertexShader,this.uniforms=ps(t.uniforms),this.uniformsGroups=ep(t.uniformsGroups),this.defines=Object.assign({},t.defines),this.wireframe=t.wireframe,this.wireframeLinewidth=t.wireframeLinewidth,this.fog=t.fog,this.lights=t.lights,this.clipping=t.clipping,this.extensions=Object.assign({},t.extensions),this.glslVersion=t.glslVersion,this}toJSON(t){let e=super.toJSON(t);e.glslVersion=this.glslVersion,e.uniforms={};for(let s in this.uniforms){let o=this.uniforms[s].value;o&&o.isTexture?e.uniforms[s]={type:"t",value:o.toJSON(t).uuid}:o&&o.isColor?e.uniforms[s]={type:"c",value:o.getHex()}:o&&o.isVector2?e.uniforms[s]={type:"v2",value:o.toArray()}:o&&o.isVector3?e.uniforms[s]={type:"v3",value:o.toArray()}:o&&o.isVector4?e.uniforms[s]={type:"v4",value:o.toArray()}:o&&o.isMatrix3?e.uniforms[s]={type:"m3",value:o.toArray()}:o&&o.isMatrix4?e.uniforms[s]={type:"m4",value:o.toArray()}:e.uniforms[s]={value:o}}Object.keys(this.defines).length>0&&(e.defines=this.defines),e.vertexShader=this.vertexShader,e.fragmentShader=this.fragmentShader,e.lights=this.lights,e.clipping=this.clipping;let i={};for(let s in this.extensions)this.extensions[s]===!0&&(i[s]=!0);return Object.keys(i).length>0&&(e.extensions=i),e}},oo=class extends Be{constructor(){super(),this.isCamera=!0,this.type="Camera",this.matrixWorldInverse=new $t,this.projectionMatrix=new $t,this.projectionMatrixInverse=new $t,this.coordinateSystem=Pn}copy(t,e){return super.copy(t,e),this.matrixWorldInverse.copy(t.matrixWorldInverse),this.projectionMatrix.copy(t.projectionMatrix),this.projectionMatrixInverse.copy(t.projectionMatrixInverse),this.coordinateSystem=t.coordinateSystem,this}getWorldDirection(t){return super.getWorldDirection(t).negate()}updateMatrixWorld(t){super.updateMatrixWorld(t),this.matrixWorldInverse.copy(this.matrixWorld).invert()}updateWorldMatrix(t,e){super.updateWorldMatrix(t,e),this.matrixWorldInverse.copy(this.matrixWorld).invert()}clone(){return new this.constructor().copy(this)}},Xn=new U,Ph=new yt,Ih=new yt,De=class extends oo{constructor(t=50,e=1,i=.1,s=2e3){super(),this.isPerspectiveCamera=!0,this.type="PerspectiveCamera",this.fov=t,this.zoom=1,this.near=i,this.far=s,this.focus=10,this.aspect=e,this.view=null,this.filmGauge=35,this.filmOffset=0,this.updateProjectionMatrix()}copy(t,e){return super.copy(t,e),this.fov=t.fov,this.zoom=t.zoom,this.near=t.near,this.far=t.far,this.focus=t.focus,this.aspect=t.aspect,this.view=t.view===null?null:Object.assign({},t.view),this.filmGauge=t.filmGauge,this.filmOffset=t.filmOffset,this}setFocalLength(t){let e=.5*this.getFilmHeight()/t;this.fov=$s*2*Math.atan(e),this.updateProjectionMatrix()}getFocalLength(){let t=Math.tan(js*.5*this.fov);return .5*this.getFilmHeight()/t}getEffectiveFOV(){return $s*2*Math.atan(Math.tan(js*.5*this.fov)/this.zoom)}getFilmWidth(){return this.filmGauge*Math.min(this.aspect,1)}getFilmHeight(){return this.filmGauge/Math.max(this.aspect,1)}getViewBounds(t,e,i){Xn.set(-1,-1,.5).applyMatrix4(this.projectionMatrixInverse),e.set(Xn.x,Xn.y).multiplyScalar(-t/Xn.z),Xn.set(1,1,.5).applyMatrix4(this.projectionMatrixInverse),i.set(Xn.x,Xn.y).multiplyScalar(-t/Xn.z)}getViewSize(t,e){return this.getViewBounds(t,Ph,Ih),e.subVectors(Ih,Ph)}setViewOffset(t,e,i,s,r,o){this.aspect=t/e,this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=t,this.view.fullHeight=e,this.view.offsetX=i,this.view.offsetY=s,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let t=this.near,e=t*Math.tan(js*.5*this.fov)/this.zoom,i=2*e,s=this.aspect*i,r=-.5*s,o=this.view;if(this.view!==null&&this.view.enabled){let l=o.fullWidth,h=o.fullHeight;r+=o.offsetX*s/l,e-=o.offsetY*i/h,s*=o.width/l,i*=o.height/h}let a=this.filmOffset;a!==0&&(r+=t*a/this.getFilmWidth()),this.projectionMatrix.makePerspective(r,r+s,e,e-i,t,this.far,this.coordinateSystem),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(t){let e=super.toJSON(t);return e.object.fov=this.fov,e.object.zoom=this.zoom,e.object.near=this.near,e.object.far=this.far,e.object.focus=this.focus,e.object.aspect=this.aspect,this.view!==null&&(e.object.view=Object.assign({},this.view)),e.object.filmGauge=this.filmGauge,e.object.filmOffset=this.filmOffset,e}},Ji=-90,Qi=1,Il=class extends Be{constructor(t,e,i){super(),this.type="CubeCamera",this.renderTarget=i,this.coordinateSystem=null,this.activeMipmapLevel=0;let s=new De(Ji,Qi,t,e);s.layers=this.layers,this.add(s);let r=new De(Ji,Qi,t,e);r.layers=this.layers,this.add(r);let o=new De(Ji,Qi,t,e);o.layers=this.layers,this.add(o);let a=new De(Ji,Qi,t,e);a.layers=this.layers,this.add(a);let l=new De(Ji,Qi,t,e);l.layers=this.layers,this.add(l);let h=new De(Ji,Qi,t,e);h.layers=this.layers,this.add(h)}updateCoordinateSystem(){let t=this.coordinateSystem,e=this.children.concat(),[i,s,r,o,a,l]=e;for(let h of e)this.remove(h);if(t===Pn)i.up.set(0,1,0),i.lookAt(1,0,0),s.up.set(0,1,0),s.lookAt(-1,0,0),r.up.set(0,0,-1),r.lookAt(0,1,0),o.up.set(0,0,1),o.lookAt(0,-1,0),a.up.set(0,1,0),a.lookAt(0,0,1),l.up.set(0,1,0),l.lookAt(0,0,-1);else if(t===$r)i.up.set(0,-1,0),i.lookAt(-1,0,0),s.up.set(0,-1,0),s.lookAt(1,0,0),r.up.set(0,0,1),r.lookAt(0,1,0),o.up.set(0,0,-1),o.lookAt(0,-1,0),a.up.set(0,-1,0),a.lookAt(0,0,1),l.up.set(0,-1,0),l.lookAt(0,0,-1);else throw new Error("THREE.CubeCamera.updateCoordinateSystem(): Invalid coordinate system: "+t);for(let h of e)this.add(h),h.updateMatrixWorld()}update(t,e){this.parent===null&&this.updateMatrixWorld();let{renderTarget:i,activeMipmapLevel:s}=this;this.coordinateSystem!==t.coordinateSystem&&(this.coordinateSystem=t.coordinateSystem,this.updateCoordinateSystem());let[r,o,a,l,h,f]=this.children,d=t.getRenderTarget(),c=t.getActiveCubeFace(),u=t.getActiveMipmapLevel(),g=t.xr.enabled;t.xr.enabled=!1;let y=i.texture.generateMipmaps;i.texture.generateMipmaps=!1,t.setRenderTarget(i,0,s),t.render(e,r),t.setRenderTarget(i,1,s),t.render(e,o),t.setRenderTarget(i,2,s),t.render(e,a),t.setRenderTarget(i,3,s),t.render(e,l),t.setRenderTarget(i,4,s),t.render(e,h),i.texture.generateMipmaps=y,t.setRenderTarget(i,5,s),t.render(e,f),t.setRenderTarget(d,c,u),t.xr.enabled=g,i.texture.needsPMREMUpdate=!0}},ao=class extends Ce{constructor(t,e,i,s,r,o,a,l,h,f){t=t!==void 0?t:[],e=e!==void 0?e:cs,super(t,e,i,s,r,o,a,l,h,f),this.isCubeTexture=!0,this.flipY=!1}get images(){return this.image}set images(t){this.image=t}},Ll=class extends ue{constructor(t=1,e={}){super(t,t,e),this.isWebGLCubeRenderTarget=!0;let i={width:t,height:t,depth:1},s=[i,i,i,i,i,i];this.texture=new ao(s,e.mapping,e.wrapS,e.wrapT,e.magFilter,e.minFilter,e.format,e.type,e.anisotropy,e.colorSpace),this.texture.isRenderTargetTexture=!0,this.texture.generateMipmaps=e.generateMipmaps!==void 0?e.generateMipmaps:!1,this.texture.minFilter=e.minFilter!==void 0?e.minFilter:Xe}fromEquirectangularTexture(t,e){this.texture.type=e.type,this.texture.colorSpace=e.colorSpace,this.texture.generateMipmaps=e.generateMipmaps,this.texture.minFilter=e.minFilter,this.texture.magFilter=e.magFilter;let i={uniforms:{tEquirect:{value:null}},vertexShader:`

				varying vec3 vWorldDirection;

				vec3 transformDirection( in vec3 dir, in mat4 matrix ) {

					return normalize( ( matrix * vec4( dir, 0.0 ) ).xyz );

				}

				void main() {

					vWorldDirection = transformDirection( position, modelMatrix );

					#include <begin_vertex>
					#include <project_vertex>

				}
			`,fragmentShader:`

				uniform sampler2D tEquirect;

				varying vec3 vWorldDirection;

				#include <common>

				void main() {

					vec3 direction = normalize( vWorldDirection );

					vec2 sampleUV = equirectUv( direction );

					gl_FragColor = texture2D( tEquirect, sampleUV );

				}
			`},s=new er(5,5,5),r=new ie({name:"CubemapFromEquirect",uniforms:ps(i.uniforms),vertexShader:i.vertexShader,fragmentShader:i.fragmentShader,side:Fe,blending:vn});r.uniforms.tEquirect.value=e;let o=new Ue(s,r),a=e.minFilter;return e.minFilter===xi&&(e.minFilter=Xe),new Il(1,10,this).update(t,o),e.minFilter=a,o.geometry.dispose(),o.material.dispose(),this}clear(t,e,i,s){let r=t.getRenderTarget();for(let o=0;o<6;o++)t.setRenderTarget(this,o),t.clear(e,i,s);t.setRenderTarget(r)}},Ua=new U,sp=new U,rp=new zt,cn=class{constructor(t=new U(1,0,0),e=0){this.isPlane=!0,this.normal=t,this.constant=e}set(t,e){return this.normal.copy(t),this.constant=e,this}setComponents(t,e,i,s){return this.normal.set(t,e,i),this.constant=s,this}setFromNormalAndCoplanarPoint(t,e){return this.normal.copy(t),this.constant=-e.dot(this.normal),this}setFromCoplanarPoints(t,e,i){let s=Ua.subVectors(i,e).cross(sp.subVectors(t,e)).normalize();return this.setFromNormalAndCoplanarPoint(s,t),this}copy(t){return this.normal.copy(t.normal),this.constant=t.constant,this}normalize(){let t=1/this.normal.length();return this.normal.multiplyScalar(t),this.constant*=t,this}negate(){return this.constant*=-1,this.normal.negate(),this}distanceToPoint(t){return this.normal.dot(t)+this.constant}distanceToSphere(t){return this.distanceToPoint(t.center)-t.radius}projectPoint(t,e){return e.copy(t).addScaledVector(this.normal,-this.distanceToPoint(t))}intersectLine(t,e){let i=t.delta(Ua),s=this.normal.dot(i);if(s===0)return this.distanceToPoint(t.start)===0?e.copy(t.start):null;let r=-(t.start.dot(this.normal)+this.constant)/s;return r<0||r>1?null:e.copy(t.start).addScaledVector(i,r)}intersectsLine(t){let e=this.distanceToPoint(t.start),i=this.distanceToPoint(t.end);return e<0&&i>0||i<0&&e>0}intersectsBox(t){return t.intersectsPlane(this)}intersectsSphere(t){return t.intersectsPlane(this)}coplanarPoint(t){return t.copy(this.normal).multiplyScalar(-this.constant)}applyMatrix4(t,e){let i=e||rp.getNormalMatrix(t),s=this.coplanarPoint(Ua).applyMatrix4(t),r=this.normal.applyMatrix3(i).normalize();return this.constant=-s.dot(r),this}translate(t){return this.constant-=t.dot(this.normal),this}equals(t){return t.normal.equals(this.normal)&&t.constant===this.constant}clone(){return new this.constructor().copy(this)}},ci=new _i,Fr=new U,nr=class{constructor(t=new cn,e=new cn,i=new cn,s=new cn,r=new cn,o=new cn){this.planes=[t,e,i,s,r,o]}set(t,e,i,s,r,o){let a=this.planes;return a[0].copy(t),a[1].copy(e),a[2].copy(i),a[3].copy(s),a[4].copy(r),a[5].copy(o),this}copy(t){let e=this.planes;for(let i=0;i<6;i++)e[i].copy(t.planes[i]);return this}setFromProjectionMatrix(t,e=Pn){let i=this.planes,s=t.elements,r=s[0],o=s[1],a=s[2],l=s[3],h=s[4],f=s[5],d=s[6],c=s[7],u=s[8],g=s[9],y=s[10],p=s[11],m=s[12],w=s[13],_=s[14],A=s[15];if(i[0].setComponents(l-r,c-h,p-u,A-m).normalize(),i[1].setComponents(l+r,c+h,p+u,A+m).normalize(),i[2].setComponents(l+o,c+f,p+g,A+w).normalize(),i[3].setComponents(l-o,c-f,p-g,A-w).normalize(),i[4].setComponents(l-a,c-d,p-y,A-_).normalize(),e===Pn)i[5].setComponents(l+a,c+d,p+y,A+_).normalize();else if(e===$r)i[5].setComponents(a,d,y,_).normalize();else throw new Error("THREE.Frustum.setFromProjectionMatrix(): Invalid coordinate system: "+e);return this}intersectsObject(t){if(t.boundingSphere!==void 0)t.boundingSphere===null&&t.computeBoundingSphere(),ci.copy(t.boundingSphere).applyMatrix4(t.matrixWorld);else{let e=t.geometry;e.boundingSphere===null&&e.computeBoundingSphere(),ci.copy(e.boundingSphere).applyMatrix4(t.matrixWorld)}return this.intersectsSphere(ci)}intersectsSprite(t){return ci.center.set(0,0,0),ci.radius=.7071067811865476,ci.applyMatrix4(t.matrixWorld),this.intersectsSphere(ci)}intersectsSphere(t){let e=this.planes,i=t.center,s=-t.radius;for(let r=0;r<6;r++)if(e[r].distanceToPoint(i)<s)return!1;return!0}intersectsBox(t){let e=this.planes;for(let i=0;i<6;i++){let s=e[i];if(Fr.x=s.normal.x>0?t.max.x:t.min.x,Fr.y=s.normal.y>0?t.max.y:t.min.y,Fr.z=s.normal.z>0?t.max.z:t.min.z,s.distanceToPoint(Fr)<0)return!1}return!0}containsPoint(t){let e=this.planes;for(let i=0;i<6;i++)if(e[i].distanceToPoint(t)<0)return!1;return!0}clone(){return new this.constructor().copy(this)}};function Au(){let n=null,t=!1,e=null,i=null;function s(r,o){e(r,o),i=n.requestAnimationFrame(s)}return{start:function(){t!==!0&&e!==null&&(i=n.requestAnimationFrame(s),t=!0)},stop:function(){n.cancelAnimationFrame(i),t=!1},setAnimationLoop:function(r){e=r},setContext:function(r){n=r}}}function op(n){let t=new WeakMap;function e(a,l){let h=a.array,f=a.usage,d=h.byteLength,c=n.createBuffer();n.bindBuffer(l,c),n.bufferData(l,h,f),a.onUploadCallback();let u;if(h instanceof Float32Array)u=n.FLOAT;else if(h instanceof Uint16Array)a.isFloat16BufferAttribute?u=n.HALF_FLOAT:u=n.UNSIGNED_SHORT;else if(h instanceof Int16Array)u=n.SHORT;else if(h instanceof Uint32Array)u=n.UNSIGNED_INT;else if(h instanceof Int32Array)u=n.INT;else if(h instanceof Int8Array)u=n.BYTE;else if(h instanceof Uint8Array)u=n.UNSIGNED_BYTE;else if(h instanceof Uint8ClampedArray)u=n.UNSIGNED_BYTE;else throw new Error("THREE.WebGLAttributes: Unsupported buffer data format: "+h);return{buffer:c,type:u,bytesPerElement:h.BYTES_PER_ELEMENT,version:a.version,size:d}}function i(a,l,h){let f=l.array,d=l.updateRanges;if(n.bindBuffer(h,a),d.length===0)n.bufferSubData(h,0,f);else{d.sort((u,g)=>u.start-g.start);let c=0;for(let u=1;u<d.length;u++){let g=d[c],y=d[u];y.start<=g.start+g.count+1?g.count=Math.max(g.count,y.start+y.count-g.start):(++c,d[c]=y)}d.length=c+1;for(let u=0,g=d.length;u<g;u++){let y=d[u];n.bufferSubData(h,y.start*f.BYTES_PER_ELEMENT,f,y.start,y.count)}l.clearUpdateRanges()}l.onUploadCallback()}function s(a){return a.isInterleavedBufferAttribute&&(a=a.data),t.get(a)}function r(a){a.isInterleavedBufferAttribute&&(a=a.data);let l=t.get(a);l&&(n.deleteBuffer(l.buffer),t.delete(a))}function o(a,l){if(a.isInterleavedBufferAttribute&&(a=a.data),a.isGLBufferAttribute){let f=t.get(a);(!f||f.version<a.version)&&t.set(a,{buffer:a.buffer,type:a.type,bytesPerElement:a.elementSize,version:a.version});return}let h=t.get(a);if(h===void 0)t.set(a,e(a,l));else if(h.version<a.version){if(h.size!==a.array.byteLength)throw new Error("THREE.WebGLAttributes: The size of the buffer attribute's array buffer does not match the original size. Resizing buffer attributes is not supported.");i(h.buffer,a,l),h.version=a.version}}return{get:s,remove:r,update:o}}var lo=class n extends un{constructor(t=1,e=1,i=1,s=1){super(),this.type="PlaneGeometry",this.parameters={width:t,height:e,widthSegments:i,heightSegments:s};let r=t/2,o=e/2,a=Math.floor(i),l=Math.floor(s),h=a+1,f=l+1,d=t/a,c=e/l,u=[],g=[],y=[],p=[];for(let m=0;m<f;m++){let w=m*c-o;for(let _=0;_<h;_++){let A=_*d-r;g.push(A,-w,0),y.push(0,0,1),p.push(_/a),p.push(1-m/l)}}for(let m=0;m<l;m++)for(let w=0;w<a;w++){let _=w+h*m,A=w+h*(m+1),I=w+1+h*(m+1),C=w+1+h*m;u.push(_,A,C),u.push(A,I,C)}this.setIndex(u),this.setAttribute("position",new ve(g,3)),this.setAttribute("normal",new ve(y,3)),this.setAttribute("uv",new ve(p,2))}copy(t){return super.copy(t),this.parameters=Object.assign({},t.parameters),this}static fromJSON(t){return new n(t.width,t.height,t.widthSegments,t.heightSegments)}},ap=`#ifdef USE_ALPHAHASH
	if ( diffuseColor.a < getAlphaHashThreshold( vPosition ) ) discard;
#endif`,lp=`#ifdef USE_ALPHAHASH
	const float ALPHA_HASH_SCALE = 0.05;
	float hash2D( vec2 value ) {
		return fract( 1.0e4 * sin( 17.0 * value.x + 0.1 * value.y ) * ( 0.1 + abs( sin( 13.0 * value.y + value.x ) ) ) );
	}
	float hash3D( vec3 value ) {
		return hash2D( vec2( hash2D( value.xy ), value.z ) );
	}
	float getAlphaHashThreshold( vec3 position ) {
		float maxDeriv = max(
			length( dFdx( position.xyz ) ),
			length( dFdy( position.xyz ) )
		);
		float pixScale = 1.0 / ( ALPHA_HASH_SCALE * maxDeriv );
		vec2 pixScales = vec2(
			exp2( floor( log2( pixScale ) ) ),
			exp2( ceil( log2( pixScale ) ) )
		);
		vec2 alpha = vec2(
			hash3D( floor( pixScales.x * position.xyz ) ),
			hash3D( floor( pixScales.y * position.xyz ) )
		);
		float lerpFactor = fract( log2( pixScale ) );
		float x = ( 1.0 - lerpFactor ) * alpha.x + lerpFactor * alpha.y;
		float a = min( lerpFactor, 1.0 - lerpFactor );
		vec3 cases = vec3(
			x * x / ( 2.0 * a * ( 1.0 - a ) ),
			( x - 0.5 * a ) / ( 1.0 - a ),
			1.0 - ( ( 1.0 - x ) * ( 1.0 - x ) / ( 2.0 * a * ( 1.0 - a ) ) )
		);
		float threshold = ( x < ( 1.0 - a ) )
			? ( ( x < a ) ? cases.x : cases.y )
			: cases.z;
		return clamp( threshold , 1.0e-6, 1.0 );
	}
#endif`,cp=`#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, vAlphaMapUv ).g;
#endif`,hp=`#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,up=`#ifdef USE_ALPHATEST
	#ifdef ALPHA_TO_COVERAGE
	diffuseColor.a = smoothstep( alphaTest, alphaTest + fwidth( diffuseColor.a ), diffuseColor.a );
	if ( diffuseColor.a == 0.0 ) discard;
	#else
	if ( diffuseColor.a < alphaTest ) discard;
	#endif
#endif`,dp=`#ifdef USE_ALPHATEST
	uniform float alphaTest;
#endif`,fp=`#ifdef USE_AOMAP
	float ambientOcclusion = ( texture2D( aoMap, vAoMapUv ).r - 1.0 ) * aoMapIntensity + 1.0;
	reflectedLight.indirectDiffuse *= ambientOcclusion;
	#if defined( USE_CLEARCOAT ) 
		clearcoatSpecularIndirect *= ambientOcclusion;
	#endif
	#if defined( USE_SHEEN ) 
		sheenSpecularIndirect *= ambientOcclusion;
	#endif
	#if defined( USE_ENVMAP ) && defined( STANDARD )
		float dotNV = saturate( dot( geometryNormal, geometryViewDir ) );
		reflectedLight.indirectSpecular *= computeSpecularOcclusion( dotNV, ambientOcclusion, material.roughness );
	#endif
#endif`,pp=`#ifdef USE_AOMAP
	uniform sampler2D aoMap;
	uniform float aoMapIntensity;
#endif`,mp=`#ifdef USE_BATCHING
	#if ! defined( GL_ANGLE_multi_draw )
	#define gl_DrawID _gl_DrawID
	uniform int _gl_DrawID;
	#endif
	uniform highp sampler2D batchingTexture;
	uniform highp usampler2D batchingIdTexture;
	mat4 getBatchingMatrix( const in float i ) {
		int size = textureSize( batchingTexture, 0 ).x;
		int j = int( i ) * 4;
		int x = j % size;
		int y = j / size;
		vec4 v1 = texelFetch( batchingTexture, ivec2( x, y ), 0 );
		vec4 v2 = texelFetch( batchingTexture, ivec2( x + 1, y ), 0 );
		vec4 v3 = texelFetch( batchingTexture, ivec2( x + 2, y ), 0 );
		vec4 v4 = texelFetch( batchingTexture, ivec2( x + 3, y ), 0 );
		return mat4( v1, v2, v3, v4 );
	}
	float getIndirectIndex( const in int i ) {
		int size = textureSize( batchingIdTexture, 0 ).x;
		int x = i % size;
		int y = i / size;
		return float( texelFetch( batchingIdTexture, ivec2( x, y ), 0 ).r );
	}
#endif
#ifdef USE_BATCHING_COLOR
	uniform sampler2D batchingColorTexture;
	vec3 getBatchingColor( const in float i ) {
		int size = textureSize( batchingColorTexture, 0 ).x;
		int j = int( i );
		int x = j % size;
		int y = j / size;
		return texelFetch( batchingColorTexture, ivec2( x, y ), 0 ).rgb;
	}
#endif`,gp=`#ifdef USE_BATCHING
	mat4 batchingMatrix = getBatchingMatrix( getIndirectIndex( gl_DrawID ) );
#endif`,xp=`vec3 transformed = vec3( position );
#ifdef USE_ALPHAHASH
	vPosition = vec3( position );
#endif`,vp=`vec3 objectNormal = vec3( normal );
#ifdef USE_TANGENT
	vec3 objectTangent = vec3( tangent.xyz );
#endif`,_p=`float G_BlinnPhong_Implicit( ) {
	return 0.25;
}
float D_BlinnPhong( const in float shininess, const in float dotNH ) {
	return RECIPROCAL_PI * ( shininess * 0.5 + 1.0 ) * pow( dotNH, shininess );
}
vec3 BRDF_BlinnPhong( const in vec3 lightDir, const in vec3 viewDir, const in vec3 normal, const in vec3 specularColor, const in float shininess ) {
	vec3 halfDir = normalize( lightDir + viewDir );
	float dotNH = saturate( dot( normal, halfDir ) );
	float dotVH = saturate( dot( viewDir, halfDir ) );
	vec3 F = F_Schlick( specularColor, 1.0, dotVH );
	float G = G_BlinnPhong_Implicit( );
	float D = D_BlinnPhong( shininess, dotNH );
	return F * ( G * D );
} // validated`,yp=`#ifdef USE_IRIDESCENCE
	const mat3 XYZ_TO_REC709 = mat3(
		 3.2404542, -0.9692660,  0.0556434,
		-1.5371385,  1.8760108, -0.2040259,
		-0.4985314,  0.0415560,  1.0572252
	);
	vec3 Fresnel0ToIor( vec3 fresnel0 ) {
		vec3 sqrtF0 = sqrt( fresnel0 );
		return ( vec3( 1.0 ) + sqrtF0 ) / ( vec3( 1.0 ) - sqrtF0 );
	}
	vec3 IorToFresnel0( vec3 transmittedIor, float incidentIor ) {
		return pow2( ( transmittedIor - vec3( incidentIor ) ) / ( transmittedIor + vec3( incidentIor ) ) );
	}
	float IorToFresnel0( float transmittedIor, float incidentIor ) {
		return pow2( ( transmittedIor - incidentIor ) / ( transmittedIor + incidentIor ));
	}
	vec3 evalSensitivity( float OPD, vec3 shift ) {
		float phase = 2.0 * PI * OPD * 1.0e-9;
		vec3 val = vec3( 5.4856e-13, 4.4201e-13, 5.2481e-13 );
		vec3 pos = vec3( 1.6810e+06, 1.7953e+06, 2.2084e+06 );
		vec3 var = vec3( 4.3278e+09, 9.3046e+09, 6.6121e+09 );
		vec3 xyz = val * sqrt( 2.0 * PI * var ) * cos( pos * phase + shift ) * exp( - pow2( phase ) * var );
		xyz.x += 9.7470e-14 * sqrt( 2.0 * PI * 4.5282e+09 ) * cos( 2.2399e+06 * phase + shift[ 0 ] ) * exp( - 4.5282e+09 * pow2( phase ) );
		xyz /= 1.0685e-7;
		vec3 rgb = XYZ_TO_REC709 * xyz;
		return rgb;
	}
	vec3 evalIridescence( float outsideIOR, float eta2, float cosTheta1, float thinFilmThickness, vec3 baseF0 ) {
		vec3 I;
		float iridescenceIOR = mix( outsideIOR, eta2, smoothstep( 0.0, 0.03, thinFilmThickness ) );
		float sinTheta2Sq = pow2( outsideIOR / iridescenceIOR ) * ( 1.0 - pow2( cosTheta1 ) );
		float cosTheta2Sq = 1.0 - sinTheta2Sq;
		if ( cosTheta2Sq < 0.0 ) {
			return vec3( 1.0 );
		}
		float cosTheta2 = sqrt( cosTheta2Sq );
		float R0 = IorToFresnel0( iridescenceIOR, outsideIOR );
		float R12 = F_Schlick( R0, 1.0, cosTheta1 );
		float T121 = 1.0 - R12;
		float phi12 = 0.0;
		if ( iridescenceIOR < outsideIOR ) phi12 = PI;
		float phi21 = PI - phi12;
		vec3 baseIOR = Fresnel0ToIor( clamp( baseF0, 0.0, 0.9999 ) );		vec3 R1 = IorToFresnel0( baseIOR, iridescenceIOR );
		vec3 R23 = F_Schlick( R1, 1.0, cosTheta2 );
		vec3 phi23 = vec3( 0.0 );
		if ( baseIOR[ 0 ] < iridescenceIOR ) phi23[ 0 ] = PI;
		if ( baseIOR[ 1 ] < iridescenceIOR ) phi23[ 1 ] = PI;
		if ( baseIOR[ 2 ] < iridescenceIOR ) phi23[ 2 ] = PI;
		float OPD = 2.0 * iridescenceIOR * thinFilmThickness * cosTheta2;
		vec3 phi = vec3( phi21 ) + phi23;
		vec3 R123 = clamp( R12 * R23, 1e-5, 0.9999 );
		vec3 r123 = sqrt( R123 );
		vec3 Rs = pow2( T121 ) * R23 / ( vec3( 1.0 ) - R123 );
		vec3 C0 = R12 + Rs;
		I = C0;
		vec3 Cm = Rs - T121;
		for ( int m = 1; m <= 2; ++ m ) {
			Cm *= r123;
			vec3 Sm = 2.0 * evalSensitivity( float( m ) * OPD, float( m ) * phi );
			I += Cm * Sm;
		}
		return max( I, vec3( 0.0 ) );
	}
#endif`,Mp=`#ifdef USE_BUMPMAP
	uniform sampler2D bumpMap;
	uniform float bumpScale;
	vec2 dHdxy_fwd() {
		vec2 dSTdx = dFdx( vBumpMapUv );
		vec2 dSTdy = dFdy( vBumpMapUv );
		float Hll = bumpScale * texture2D( bumpMap, vBumpMapUv ).x;
		float dBx = bumpScale * texture2D( bumpMap, vBumpMapUv + dSTdx ).x - Hll;
		float dBy = bumpScale * texture2D( bumpMap, vBumpMapUv + dSTdy ).x - Hll;
		return vec2( dBx, dBy );
	}
	vec3 perturbNormalArb( vec3 surf_pos, vec3 surf_norm, vec2 dHdxy, float faceDirection ) {
		vec3 vSigmaX = normalize( dFdx( surf_pos.xyz ) );
		vec3 vSigmaY = normalize( dFdy( surf_pos.xyz ) );
		vec3 vN = surf_norm;
		vec3 R1 = cross( vSigmaY, vN );
		vec3 R2 = cross( vN, vSigmaX );
		float fDet = dot( vSigmaX, R1 ) * faceDirection;
		vec3 vGrad = sign( fDet ) * ( dHdxy.x * R1 + dHdxy.y * R2 );
		return normalize( abs( fDet ) * surf_norm - vGrad );
	}
#endif`,Sp=`#if NUM_CLIPPING_PLANES > 0
	vec4 plane;
	#ifdef ALPHA_TO_COVERAGE
		float distanceToPlane, distanceGradient;
		float clipOpacity = 1.0;
		#pragma unroll_loop_start
		for ( int i = 0; i < UNION_CLIPPING_PLANES; i ++ ) {
			plane = clippingPlanes[ i ];
			distanceToPlane = - dot( vClipPosition, plane.xyz ) + plane.w;
			distanceGradient = fwidth( distanceToPlane ) / 2.0;
			clipOpacity *= smoothstep( - distanceGradient, distanceGradient, distanceToPlane );
			if ( clipOpacity == 0.0 ) discard;
		}
		#pragma unroll_loop_end
		#if UNION_CLIPPING_PLANES < NUM_CLIPPING_PLANES
			float unionClipOpacity = 1.0;
			#pragma unroll_loop_start
			for ( int i = UNION_CLIPPING_PLANES; i < NUM_CLIPPING_PLANES; i ++ ) {
				plane = clippingPlanes[ i ];
				distanceToPlane = - dot( vClipPosition, plane.xyz ) + plane.w;
				distanceGradient = fwidth( distanceToPlane ) / 2.0;
				unionClipOpacity *= 1.0 - smoothstep( - distanceGradient, distanceGradient, distanceToPlane );
			}
			#pragma unroll_loop_end
			clipOpacity *= 1.0 - unionClipOpacity;
		#endif
		diffuseColor.a *= clipOpacity;
		if ( diffuseColor.a == 0.0 ) discard;
	#else
		#pragma unroll_loop_start
		for ( int i = 0; i < UNION_CLIPPING_PLANES; i ++ ) {
			plane = clippingPlanes[ i ];
			if ( dot( vClipPosition, plane.xyz ) > plane.w ) discard;
		}
		#pragma unroll_loop_end
		#if UNION_CLIPPING_PLANES < NUM_CLIPPING_PLANES
			bool clipped = true;
			#pragma unroll_loop_start
			for ( int i = UNION_CLIPPING_PLANES; i < NUM_CLIPPING_PLANES; i ++ ) {
				plane = clippingPlanes[ i ];
				clipped = ( dot( vClipPosition, plane.xyz ) > plane.w ) && clipped;
			}
			#pragma unroll_loop_end
			if ( clipped ) discard;
		#endif
	#endif
#endif`,bp=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
	uniform vec4 clippingPlanes[ NUM_CLIPPING_PLANES ];
#endif`,wp=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
#endif`,Ap=`#if NUM_CLIPPING_PLANES > 0
	vClipPosition = - mvPosition.xyz;
#endif`,Ep=`#if defined( USE_COLOR_ALPHA )
	diffuseColor *= vColor;
#elif defined( USE_COLOR )
	diffuseColor.rgb *= vColor;
#endif`,Tp=`#if defined( USE_COLOR_ALPHA )
	varying vec4 vColor;
#elif defined( USE_COLOR )
	varying vec3 vColor;
#endif`,Cp=`#if defined( USE_COLOR_ALPHA )
	varying vec4 vColor;
#elif defined( USE_COLOR ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	varying vec3 vColor;
#endif`,Rp=`#if defined( USE_COLOR_ALPHA )
	vColor = vec4( 1.0 );
#elif defined( USE_COLOR ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	vColor = vec3( 1.0 );
#endif
#ifdef USE_COLOR
	vColor *= color;
#endif
#ifdef USE_INSTANCING_COLOR
	vColor.xyz *= instanceColor.xyz;
#endif
#ifdef USE_BATCHING_COLOR
	vec3 batchingColor = getBatchingColor( getIndirectIndex( gl_DrawID ) );
	vColor.xyz *= batchingColor.xyz;
#endif`,Pp=`#define PI 3.141592653589793
#define PI2 6.283185307179586
#define PI_HALF 1.5707963267948966
#define RECIPROCAL_PI 0.3183098861837907
#define RECIPROCAL_PI2 0.15915494309189535
#define EPSILON 1e-6
#ifndef saturate
#define saturate( a ) clamp( a, 0.0, 1.0 )
#endif
#define whiteComplement( a ) ( 1.0 - saturate( a ) )
float pow2( const in float x ) { return x*x; }
vec3 pow2( const in vec3 x ) { return x*x; }
float pow3( const in float x ) { return x*x*x; }
float pow4( const in float x ) { float x2 = x*x; return x2*x2; }
float max3( const in vec3 v ) { return max( max( v.x, v.y ), v.z ); }
float average( const in vec3 v ) { return dot( v, vec3( 0.3333333 ) ); }
highp float rand( const in vec2 uv ) {
	const highp float a = 12.9898, b = 78.233, c = 43758.5453;
	highp float dt = dot( uv.xy, vec2( a,b ) ), sn = mod( dt, PI );
	return fract( sin( sn ) * c );
}
#ifdef HIGH_PRECISION
	float precisionSafeLength( vec3 v ) { return length( v ); }
#else
	float precisionSafeLength( vec3 v ) {
		float maxComponent = max3( abs( v ) );
		return length( v / maxComponent ) * maxComponent;
	}
#endif
struct IncidentLight {
	vec3 color;
	vec3 direction;
	bool visible;
};
struct ReflectedLight {
	vec3 directDiffuse;
	vec3 directSpecular;
	vec3 indirectDiffuse;
	vec3 indirectSpecular;
};
#ifdef USE_ALPHAHASH
	varying vec3 vPosition;
#endif
vec3 transformDirection( in vec3 dir, in mat4 matrix ) {
	return normalize( ( matrix * vec4( dir, 0.0 ) ).xyz );
}
vec3 inverseTransformDirection( in vec3 dir, in mat4 matrix ) {
	return normalize( ( vec4( dir, 0.0 ) * matrix ).xyz );
}
mat3 transposeMat3( const in mat3 m ) {
	mat3 tmp;
	tmp[ 0 ] = vec3( m[ 0 ].x, m[ 1 ].x, m[ 2 ].x );
	tmp[ 1 ] = vec3( m[ 0 ].y, m[ 1 ].y, m[ 2 ].y );
	tmp[ 2 ] = vec3( m[ 0 ].z, m[ 1 ].z, m[ 2 ].z );
	return tmp;
}
bool isPerspectiveMatrix( mat4 m ) {
	return m[ 2 ][ 3 ] == - 1.0;
}
vec2 equirectUv( in vec3 dir ) {
	float u = atan( dir.z, dir.x ) * RECIPROCAL_PI2 + 0.5;
	float v = asin( clamp( dir.y, - 1.0, 1.0 ) ) * RECIPROCAL_PI + 0.5;
	return vec2( u, v );
}
vec3 BRDF_Lambert( const in vec3 diffuseColor ) {
	return RECIPROCAL_PI * diffuseColor;
}
vec3 F_Schlick( const in vec3 f0, const in float f90, const in float dotVH ) {
	float fresnel = exp2( ( - 5.55473 * dotVH - 6.98316 ) * dotVH );
	return f0 * ( 1.0 - fresnel ) + ( f90 * fresnel );
}
float F_Schlick( const in float f0, const in float f90, const in float dotVH ) {
	float fresnel = exp2( ( - 5.55473 * dotVH - 6.98316 ) * dotVH );
	return f0 * ( 1.0 - fresnel ) + ( f90 * fresnel );
} // validated`,Ip=`#ifdef ENVMAP_TYPE_CUBE_UV
	#define cubeUV_minMipLevel 4.0
	#define cubeUV_minTileSize 16.0
	float getFace( vec3 direction ) {
		vec3 absDirection = abs( direction );
		float face = - 1.0;
		if ( absDirection.x > absDirection.z ) {
			if ( absDirection.x > absDirection.y )
				face = direction.x > 0.0 ? 0.0 : 3.0;
			else
				face = direction.y > 0.0 ? 1.0 : 4.0;
		} else {
			if ( absDirection.z > absDirection.y )
				face = direction.z > 0.0 ? 2.0 : 5.0;
			else
				face = direction.y > 0.0 ? 1.0 : 4.0;
		}
		return face;
	}
	vec2 getUV( vec3 direction, float face ) {
		vec2 uv;
		if ( face == 0.0 ) {
			uv = vec2( direction.z, direction.y ) / abs( direction.x );
		} else if ( face == 1.0 ) {
			uv = vec2( - direction.x, - direction.z ) / abs( direction.y );
		} else if ( face == 2.0 ) {
			uv = vec2( - direction.x, direction.y ) / abs( direction.z );
		} else if ( face == 3.0 ) {
			uv = vec2( - direction.z, direction.y ) / abs( direction.x );
		} else if ( face == 4.0 ) {
			uv = vec2( - direction.x, direction.z ) / abs( direction.y );
		} else {
			uv = vec2( direction.x, direction.y ) / abs( direction.z );
		}
		return 0.5 * ( uv + 1.0 );
	}
	vec3 bilinearCubeUV( sampler2D envMap, vec3 direction, float mipInt ) {
		float face = getFace( direction );
		float filterInt = max( cubeUV_minMipLevel - mipInt, 0.0 );
		mipInt = max( mipInt, cubeUV_minMipLevel );
		float faceSize = exp2( mipInt );
		highp vec2 uv = getUV( direction, face ) * ( faceSize - 2.0 ) + 1.0;
		if ( face > 2.0 ) {
			uv.y += faceSize;
			face -= 3.0;
		}
		uv.x += face * faceSize;
		uv.x += filterInt * 3.0 * cubeUV_minTileSize;
		uv.y += 4.0 * ( exp2( CUBEUV_MAX_MIP ) - faceSize );
		uv.x *= CUBEUV_TEXEL_WIDTH;
		uv.y *= CUBEUV_TEXEL_HEIGHT;
		#ifdef texture2DGradEXT
			return texture2DGradEXT( envMap, uv, vec2( 0.0 ), vec2( 0.0 ) ).rgb;
		#else
			return texture2D( envMap, uv ).rgb;
		#endif
	}
	#define cubeUV_r0 1.0
	#define cubeUV_m0 - 2.0
	#define cubeUV_r1 0.8
	#define cubeUV_m1 - 1.0
	#define cubeUV_r4 0.4
	#define cubeUV_m4 2.0
	#define cubeUV_r5 0.305
	#define cubeUV_m5 3.0
	#define cubeUV_r6 0.21
	#define cubeUV_m6 4.0
	float roughnessToMip( float roughness ) {
		float mip = 0.0;
		if ( roughness >= cubeUV_r1 ) {
			mip = ( cubeUV_r0 - roughness ) * ( cubeUV_m1 - cubeUV_m0 ) / ( cubeUV_r0 - cubeUV_r1 ) + cubeUV_m0;
		} else if ( roughness >= cubeUV_r4 ) {
			mip = ( cubeUV_r1 - roughness ) * ( cubeUV_m4 - cubeUV_m1 ) / ( cubeUV_r1 - cubeUV_r4 ) + cubeUV_m1;
		} else if ( roughness >= cubeUV_r5 ) {
			mip = ( cubeUV_r4 - roughness ) * ( cubeUV_m5 - cubeUV_m4 ) / ( cubeUV_r4 - cubeUV_r5 ) + cubeUV_m4;
		} else if ( roughness >= cubeUV_r6 ) {
			mip = ( cubeUV_r5 - roughness ) * ( cubeUV_m6 - cubeUV_m5 ) / ( cubeUV_r5 - cubeUV_r6 ) + cubeUV_m5;
		} else {
			mip = - 2.0 * log2( 1.16 * roughness );		}
		return mip;
	}
	vec4 textureCubeUV( sampler2D envMap, vec3 sampleDir, float roughness ) {
		float mip = clamp( roughnessToMip( roughness ), cubeUV_m0, CUBEUV_MAX_MIP );
		float mipF = fract( mip );
		float mipInt = floor( mip );
		vec3 color0 = bilinearCubeUV( envMap, sampleDir, mipInt );
		if ( mipF == 0.0 ) {
			return vec4( color0, 1.0 );
		} else {
			vec3 color1 = bilinearCubeUV( envMap, sampleDir, mipInt + 1.0 );
			return vec4( mix( color0, color1, mipF ), 1.0 );
		}
	}
#endif`,Lp=`vec3 transformedNormal = objectNormal;
#ifdef USE_TANGENT
	vec3 transformedTangent = objectTangent;
#endif
#ifdef USE_BATCHING
	mat3 bm = mat3( batchingMatrix );
	transformedNormal /= vec3( dot( bm[ 0 ], bm[ 0 ] ), dot( bm[ 1 ], bm[ 1 ] ), dot( bm[ 2 ], bm[ 2 ] ) );
	transformedNormal = bm * transformedNormal;
	#ifdef USE_TANGENT
		transformedTangent = bm * transformedTangent;
	#endif
#endif
#ifdef USE_INSTANCING
	mat3 im = mat3( instanceMatrix );
	transformedNormal /= vec3( dot( im[ 0 ], im[ 0 ] ), dot( im[ 1 ], im[ 1 ] ), dot( im[ 2 ], im[ 2 ] ) );
	transformedNormal = im * transformedNormal;
	#ifdef USE_TANGENT
		transformedTangent = im * transformedTangent;
	#endif
#endif
transformedNormal = normalMatrix * transformedNormal;
#ifdef FLIP_SIDED
	transformedNormal = - transformedNormal;
#endif
#ifdef USE_TANGENT
	transformedTangent = ( modelViewMatrix * vec4( transformedTangent, 0.0 ) ).xyz;
	#ifdef FLIP_SIDED
		transformedTangent = - transformedTangent;
	#endif
#endif`,Dp=`#ifdef USE_DISPLACEMENTMAP
	uniform sampler2D displacementMap;
	uniform float displacementScale;
	uniform float displacementBias;
#endif`,Up=`#ifdef USE_DISPLACEMENTMAP
	transformed += normalize( objectNormal ) * ( texture2D( displacementMap, vDisplacementMapUv ).x * displacementScale + displacementBias );
#endif`,Np=`#ifdef USE_EMISSIVEMAP
	vec4 emissiveColor = texture2D( emissiveMap, vEmissiveMapUv );
	totalEmissiveRadiance *= emissiveColor.rgb;
#endif`,Op=`#ifdef USE_EMISSIVEMAP
	uniform sampler2D emissiveMap;
#endif`,Fp="gl_FragColor = linearToOutputTexel( gl_FragColor );",Bp=`
const mat3 LINEAR_SRGB_TO_LINEAR_DISPLAY_P3 = mat3(
	vec3( 0.8224621, 0.177538, 0.0 ),
	vec3( 0.0331941, 0.9668058, 0.0 ),
	vec3( 0.0170827, 0.0723974, 0.9105199 )
);
const mat3 LINEAR_DISPLAY_P3_TO_LINEAR_SRGB = mat3(
	vec3( 1.2249401, - 0.2249404, 0.0 ),
	vec3( - 0.0420569, 1.0420571, 0.0 ),
	vec3( - 0.0196376, - 0.0786361, 1.0982735 )
);
vec4 LinearSRGBToLinearDisplayP3( in vec4 value ) {
	return vec4( value.rgb * LINEAR_SRGB_TO_LINEAR_DISPLAY_P3, value.a );
}
vec4 LinearDisplayP3ToLinearSRGB( in vec4 value ) {
	return vec4( value.rgb * LINEAR_DISPLAY_P3_TO_LINEAR_SRGB, value.a );
}
vec4 LinearTransferOETF( in vec4 value ) {
	return value;
}
vec4 sRGBTransferOETF( in vec4 value ) {
	return vec4( mix( pow( value.rgb, vec3( 0.41666 ) ) * 1.055 - vec3( 0.055 ), value.rgb * 12.92, vec3( lessThanEqual( value.rgb, vec3( 0.0031308 ) ) ) ), value.a );
}`,zp=`#ifdef USE_ENVMAP
	#ifdef ENV_WORLDPOS
		vec3 cameraToFrag;
		if ( isOrthographic ) {
			cameraToFrag = normalize( vec3( - viewMatrix[ 0 ][ 2 ], - viewMatrix[ 1 ][ 2 ], - viewMatrix[ 2 ][ 2 ] ) );
		} else {
			cameraToFrag = normalize( vWorldPosition - cameraPosition );
		}
		vec3 worldNormal = inverseTransformDirection( normal, viewMatrix );
		#ifdef ENVMAP_MODE_REFLECTION
			vec3 reflectVec = reflect( cameraToFrag, worldNormal );
		#else
			vec3 reflectVec = refract( cameraToFrag, worldNormal, refractionRatio );
		#endif
	#else
		vec3 reflectVec = vReflect;
	#endif
	#ifdef ENVMAP_TYPE_CUBE
		vec4 envColor = textureCube( envMap, envMapRotation * vec3( flipEnvMap * reflectVec.x, reflectVec.yz ) );
	#else
		vec4 envColor = vec4( 0.0 );
	#endif
	#ifdef ENVMAP_BLENDING_MULTIPLY
		outgoingLight = mix( outgoingLight, outgoingLight * envColor.xyz, specularStrength * reflectivity );
	#elif defined( ENVMAP_BLENDING_MIX )
		outgoingLight = mix( outgoingLight, envColor.xyz, specularStrength * reflectivity );
	#elif defined( ENVMAP_BLENDING_ADD )
		outgoingLight += envColor.xyz * specularStrength * reflectivity;
	#endif
#endif`,kp=`#ifdef USE_ENVMAP
	uniform float envMapIntensity;
	uniform float flipEnvMap;
	uniform mat3 envMapRotation;
	#ifdef ENVMAP_TYPE_CUBE
		uniform samplerCube envMap;
	#else
		uniform sampler2D envMap;
	#endif
	
#endif`,Hp=`#ifdef USE_ENVMAP
	uniform float reflectivity;
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		varying vec3 vWorldPosition;
		uniform float refractionRatio;
	#else
		varying vec3 vReflect;
	#endif
#endif`,Vp=`#ifdef USE_ENVMAP
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		
		varying vec3 vWorldPosition;
	#else
		varying vec3 vReflect;
		uniform float refractionRatio;
	#endif
#endif`,Gp=`#ifdef USE_ENVMAP
	#ifdef ENV_WORLDPOS
		vWorldPosition = worldPosition.xyz;
	#else
		vec3 cameraToVertex;
		if ( isOrthographic ) {
			cameraToVertex = normalize( vec3( - viewMatrix[ 0 ][ 2 ], - viewMatrix[ 1 ][ 2 ], - viewMatrix[ 2 ][ 2 ] ) );
		} else {
			cameraToVertex = normalize( worldPosition.xyz - cameraPosition );
		}
		vec3 worldNormal = inverseTransformDirection( transformedNormal, viewMatrix );
		#ifdef ENVMAP_MODE_REFLECTION
			vReflect = reflect( cameraToVertex, worldNormal );
		#else
			vReflect = refract( cameraToVertex, worldNormal, refractionRatio );
		#endif
	#endif
#endif`,Wp=`#ifdef USE_FOG
	vFogDepth = - mvPosition.z;
#endif`,Xp=`#ifdef USE_FOG
	varying float vFogDepth;
#endif`,qp=`#ifdef USE_FOG
	#ifdef FOG_EXP2
		float fogFactor = 1.0 - exp( - fogDensity * fogDensity * vFogDepth * vFogDepth );
	#else
		float fogFactor = smoothstep( fogNear, fogFar, vFogDepth );
	#endif
	gl_FragColor.rgb = mix( gl_FragColor.rgb, fogColor, fogFactor );
#endif`,Yp=`#ifdef USE_FOG
	uniform vec3 fogColor;
	varying float vFogDepth;
	#ifdef FOG_EXP2
		uniform float fogDensity;
	#else
		uniform float fogNear;
		uniform float fogFar;
	#endif
#endif`,Zp=`#ifdef USE_GRADIENTMAP
	uniform sampler2D gradientMap;
#endif
vec3 getGradientIrradiance( vec3 normal, vec3 lightDirection ) {
	float dotNL = dot( normal, lightDirection );
	vec2 coord = vec2( dotNL * 0.5 + 0.5, 0.0 );
	#ifdef USE_GRADIENTMAP
		return vec3( texture2D( gradientMap, coord ).r );
	#else
		vec2 fw = fwidth( coord ) * 0.5;
		return mix( vec3( 0.7 ), vec3( 1.0 ), smoothstep( 0.7 - fw.x, 0.7 + fw.x, coord.x ) );
	#endif
}`,jp=`#ifdef USE_LIGHTMAP
	uniform sampler2D lightMap;
	uniform float lightMapIntensity;
#endif`,Kp=`LambertMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularStrength = specularStrength;`,Jp=`varying vec3 vViewPosition;
struct LambertMaterial {
	vec3 diffuseColor;
	float specularStrength;
};
void RE_Direct_Lambert( const in IncidentLight directLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in LambertMaterial material, inout ReflectedLight reflectedLight ) {
	float dotNL = saturate( dot( geometryNormal, directLight.direction ) );
	vec3 irradiance = dotNL * directLight.color;
	reflectedLight.directDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
void RE_IndirectDiffuse_Lambert( const in vec3 irradiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in LambertMaterial material, inout ReflectedLight reflectedLight ) {
	reflectedLight.indirectDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
#define RE_Direct				RE_Direct_Lambert
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Lambert`,Qp=`uniform bool receiveShadow;
uniform vec3 ambientLightColor;
#if defined( USE_LIGHT_PROBES )
	uniform vec3 lightProbe[ 9 ];
#endif
vec3 shGetIrradianceAt( in vec3 normal, in vec3 shCoefficients[ 9 ] ) {
	float x = normal.x, y = normal.y, z = normal.z;
	vec3 result = shCoefficients[ 0 ] * 0.886227;
	result += shCoefficients[ 1 ] * 2.0 * 0.511664 * y;
	result += shCoefficients[ 2 ] * 2.0 * 0.511664 * z;
	result += shCoefficients[ 3 ] * 2.0 * 0.511664 * x;
	result += shCoefficients[ 4 ] * 2.0 * 0.429043 * x * y;
	result += shCoefficients[ 5 ] * 2.0 * 0.429043 * y * z;
	result += shCoefficients[ 6 ] * ( 0.743125 * z * z - 0.247708 );
	result += shCoefficients[ 7 ] * 2.0 * 0.429043 * x * z;
	result += shCoefficients[ 8 ] * 0.429043 * ( x * x - y * y );
	return result;
}
vec3 getLightProbeIrradiance( const in vec3 lightProbe[ 9 ], const in vec3 normal ) {
	vec3 worldNormal = inverseTransformDirection( normal, viewMatrix );
	vec3 irradiance = shGetIrradianceAt( worldNormal, lightProbe );
	return irradiance;
}
vec3 getAmbientLightIrradiance( const in vec3 ambientLightColor ) {
	vec3 irradiance = ambientLightColor;
	return irradiance;
}
float getDistanceAttenuation( const in float lightDistance, const in float cutoffDistance, const in float decayExponent ) {
	float distanceFalloff = 1.0 / max( pow( lightDistance, decayExponent ), 0.01 );
	if ( cutoffDistance > 0.0 ) {
		distanceFalloff *= pow2( saturate( 1.0 - pow4( lightDistance / cutoffDistance ) ) );
	}
	return distanceFalloff;
}
float getSpotAttenuation( const in float coneCosine, const in float penumbraCosine, const in float angleCosine ) {
	return smoothstep( coneCosine, penumbraCosine, angleCosine );
}
#if NUM_DIR_LIGHTS > 0
	struct DirectionalLight {
		vec3 direction;
		vec3 color;
	};
	uniform DirectionalLight directionalLights[ NUM_DIR_LIGHTS ];
	void getDirectionalLightInfo( const in DirectionalLight directionalLight, out IncidentLight light ) {
		light.color = directionalLight.color;
		light.direction = directionalLight.direction;
		light.visible = true;
	}
#endif
#if NUM_POINT_LIGHTS > 0
	struct PointLight {
		vec3 position;
		vec3 color;
		float distance;
		float decay;
	};
	uniform PointLight pointLights[ NUM_POINT_LIGHTS ];
	void getPointLightInfo( const in PointLight pointLight, const in vec3 geometryPosition, out IncidentLight light ) {
		vec3 lVector = pointLight.position - geometryPosition;
		light.direction = normalize( lVector );
		float lightDistance = length( lVector );
		light.color = pointLight.color;
		light.color *= getDistanceAttenuation( lightDistance, pointLight.distance, pointLight.decay );
		light.visible = ( light.color != vec3( 0.0 ) );
	}
#endif
#if NUM_SPOT_LIGHTS > 0
	struct SpotLight {
		vec3 position;
		vec3 direction;
		vec3 color;
		float distance;
		float decay;
		float coneCos;
		float penumbraCos;
	};
	uniform SpotLight spotLights[ NUM_SPOT_LIGHTS ];
	void getSpotLightInfo( const in SpotLight spotLight, const in vec3 geometryPosition, out IncidentLight light ) {
		vec3 lVector = spotLight.position - geometryPosition;
		light.direction = normalize( lVector );
		float angleCos = dot( light.direction, spotLight.direction );
		float spotAttenuation = getSpotAttenuation( spotLight.coneCos, spotLight.penumbraCos, angleCos );
		if ( spotAttenuation > 0.0 ) {
			float lightDistance = length( lVector );
			light.color = spotLight.color * spotAttenuation;
			light.color *= getDistanceAttenuation( lightDistance, spotLight.distance, spotLight.decay );
			light.visible = ( light.color != vec3( 0.0 ) );
		} else {
			light.color = vec3( 0.0 );
			light.visible = false;
		}
	}
#endif
#if NUM_RECT_AREA_LIGHTS > 0
	struct RectAreaLight {
		vec3 color;
		vec3 position;
		vec3 halfWidth;
		vec3 halfHeight;
	};
	uniform sampler2D ltc_1;	uniform sampler2D ltc_2;
	uniform RectAreaLight rectAreaLights[ NUM_RECT_AREA_LIGHTS ];
#endif
#if NUM_HEMI_LIGHTS > 0
	struct HemisphereLight {
		vec3 direction;
		vec3 skyColor;
		vec3 groundColor;
	};
	uniform HemisphereLight hemisphereLights[ NUM_HEMI_LIGHTS ];
	vec3 getHemisphereLightIrradiance( const in HemisphereLight hemiLight, const in vec3 normal ) {
		float dotNL = dot( normal, hemiLight.direction );
		float hemiDiffuseWeight = 0.5 * dotNL + 0.5;
		vec3 irradiance = mix( hemiLight.groundColor, hemiLight.skyColor, hemiDiffuseWeight );
		return irradiance;
	}
#endif`,$p=`#ifdef USE_ENVMAP
	vec3 getIBLIrradiance( const in vec3 normal ) {
		#ifdef ENVMAP_TYPE_CUBE_UV
			vec3 worldNormal = inverseTransformDirection( normal, viewMatrix );
			vec4 envMapColor = textureCubeUV( envMap, envMapRotation * worldNormal, 1.0 );
			return PI * envMapColor.rgb * envMapIntensity;
		#else
			return vec3( 0.0 );
		#endif
	}
	vec3 getIBLRadiance( const in vec3 viewDir, const in vec3 normal, const in float roughness ) {
		#ifdef ENVMAP_TYPE_CUBE_UV
			vec3 reflectVec = reflect( - viewDir, normal );
			reflectVec = normalize( mix( reflectVec, normal, roughness * roughness) );
			reflectVec = inverseTransformDirection( reflectVec, viewMatrix );
			vec4 envMapColor = textureCubeUV( envMap, envMapRotation * reflectVec, roughness );
			return envMapColor.rgb * envMapIntensity;
		#else
			return vec3( 0.0 );
		#endif
	}
	#ifdef USE_ANISOTROPY
		vec3 getIBLAnisotropyRadiance( const in vec3 viewDir, const in vec3 normal, const in float roughness, const in vec3 bitangent, const in float anisotropy ) {
			#ifdef ENVMAP_TYPE_CUBE_UV
				vec3 bentNormal = cross( bitangent, viewDir );
				bentNormal = normalize( cross( bentNormal, bitangent ) );
				bentNormal = normalize( mix( bentNormal, normal, pow2( pow2( 1.0 - anisotropy * ( 1.0 - roughness ) ) ) ) );
				return getIBLRadiance( viewDir, bentNormal, roughness );
			#else
				return vec3( 0.0 );
			#endif
		}
	#endif
#endif`,tm=`ToonMaterial material;
material.diffuseColor = diffuseColor.rgb;`,em=`varying vec3 vViewPosition;
struct ToonMaterial {
	vec3 diffuseColor;
};
void RE_Direct_Toon( const in IncidentLight directLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in ToonMaterial material, inout ReflectedLight reflectedLight ) {
	vec3 irradiance = getGradientIrradiance( geometryNormal, directLight.direction ) * directLight.color;
	reflectedLight.directDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
void RE_IndirectDiffuse_Toon( const in vec3 irradiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in ToonMaterial material, inout ReflectedLight reflectedLight ) {
	reflectedLight.indirectDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
#define RE_Direct				RE_Direct_Toon
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Toon`,nm=`BlinnPhongMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularColor = specular;
material.specularShininess = shininess;
material.specularStrength = specularStrength;`,im=`varying vec3 vViewPosition;
struct BlinnPhongMaterial {
	vec3 diffuseColor;
	vec3 specularColor;
	float specularShininess;
	float specularStrength;
};
void RE_Direct_BlinnPhong( const in IncidentLight directLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in BlinnPhongMaterial material, inout ReflectedLight reflectedLight ) {
	float dotNL = saturate( dot( geometryNormal, directLight.direction ) );
	vec3 irradiance = dotNL * directLight.color;
	reflectedLight.directDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
	reflectedLight.directSpecular += irradiance * BRDF_BlinnPhong( directLight.direction, geometryViewDir, geometryNormal, material.specularColor, material.specularShininess ) * material.specularStrength;
}
void RE_IndirectDiffuse_BlinnPhong( const in vec3 irradiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in BlinnPhongMaterial material, inout ReflectedLight reflectedLight ) {
	reflectedLight.indirectDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
#define RE_Direct				RE_Direct_BlinnPhong
#define RE_IndirectDiffuse		RE_IndirectDiffuse_BlinnPhong`,sm=`PhysicalMaterial material;
material.diffuseColor = diffuseColor.rgb * ( 1.0 - metalnessFactor );
vec3 dxy = max( abs( dFdx( nonPerturbedNormal ) ), abs( dFdy( nonPerturbedNormal ) ) );
float geometryRoughness = max( max( dxy.x, dxy.y ), dxy.z );
material.roughness = max( roughnessFactor, 0.0525 );material.roughness += geometryRoughness;
material.roughness = min( material.roughness, 1.0 );
#ifdef IOR
	material.ior = ior;
	#ifdef USE_SPECULAR
		float specularIntensityFactor = specularIntensity;
		vec3 specularColorFactor = specularColor;
		#ifdef USE_SPECULAR_COLORMAP
			specularColorFactor *= texture2D( specularColorMap, vSpecularColorMapUv ).rgb;
		#endif
		#ifdef USE_SPECULAR_INTENSITYMAP
			specularIntensityFactor *= texture2D( specularIntensityMap, vSpecularIntensityMapUv ).a;
		#endif
		material.specularF90 = mix( specularIntensityFactor, 1.0, metalnessFactor );
	#else
		float specularIntensityFactor = 1.0;
		vec3 specularColorFactor = vec3( 1.0 );
		material.specularF90 = 1.0;
	#endif
	material.specularColor = mix( min( pow2( ( material.ior - 1.0 ) / ( material.ior + 1.0 ) ) * specularColorFactor, vec3( 1.0 ) ) * specularIntensityFactor, diffuseColor.rgb, metalnessFactor );
#else
	material.specularColor = mix( vec3( 0.04 ), diffuseColor.rgb, metalnessFactor );
	material.specularF90 = 1.0;
#endif
#ifdef USE_CLEARCOAT
	material.clearcoat = clearcoat;
	material.clearcoatRoughness = clearcoatRoughness;
	material.clearcoatF0 = vec3( 0.04 );
	material.clearcoatF90 = 1.0;
	#ifdef USE_CLEARCOATMAP
		material.clearcoat *= texture2D( clearcoatMap, vClearcoatMapUv ).x;
	#endif
	#ifdef USE_CLEARCOAT_ROUGHNESSMAP
		material.clearcoatRoughness *= texture2D( clearcoatRoughnessMap, vClearcoatRoughnessMapUv ).y;
	#endif
	material.clearcoat = saturate( material.clearcoat );	material.clearcoatRoughness = max( material.clearcoatRoughness, 0.0525 );
	material.clearcoatRoughness += geometryRoughness;
	material.clearcoatRoughness = min( material.clearcoatRoughness, 1.0 );
#endif
#ifdef USE_DISPERSION
	material.dispersion = dispersion;
#endif
#ifdef USE_IRIDESCENCE
	material.iridescence = iridescence;
	material.iridescenceIOR = iridescenceIOR;
	#ifdef USE_IRIDESCENCEMAP
		material.iridescence *= texture2D( iridescenceMap, vIridescenceMapUv ).r;
	#endif
	#ifdef USE_IRIDESCENCE_THICKNESSMAP
		material.iridescenceThickness = (iridescenceThicknessMaximum - iridescenceThicknessMinimum) * texture2D( iridescenceThicknessMap, vIridescenceThicknessMapUv ).g + iridescenceThicknessMinimum;
	#else
		material.iridescenceThickness = iridescenceThicknessMaximum;
	#endif
#endif
#ifdef USE_SHEEN
	material.sheenColor = sheenColor;
	#ifdef USE_SHEEN_COLORMAP
		material.sheenColor *= texture2D( sheenColorMap, vSheenColorMapUv ).rgb;
	#endif
	material.sheenRoughness = clamp( sheenRoughness, 0.07, 1.0 );
	#ifdef USE_SHEEN_ROUGHNESSMAP
		material.sheenRoughness *= texture2D( sheenRoughnessMap, vSheenRoughnessMapUv ).a;
	#endif
#endif
#ifdef USE_ANISOTROPY
	#ifdef USE_ANISOTROPYMAP
		mat2 anisotropyMat = mat2( anisotropyVector.x, anisotropyVector.y, - anisotropyVector.y, anisotropyVector.x );
		vec3 anisotropyPolar = texture2D( anisotropyMap, vAnisotropyMapUv ).rgb;
		vec2 anisotropyV = anisotropyMat * normalize( 2.0 * anisotropyPolar.rg - vec2( 1.0 ) ) * anisotropyPolar.b;
	#else
		vec2 anisotropyV = anisotropyVector;
	#endif
	material.anisotropy = length( anisotropyV );
	if( material.anisotropy == 0.0 ) {
		anisotropyV = vec2( 1.0, 0.0 );
	} else {
		anisotropyV /= material.anisotropy;
		material.anisotropy = saturate( material.anisotropy );
	}
	material.alphaT = mix( pow2( material.roughness ), 1.0, pow2( material.anisotropy ) );
	material.anisotropyT = tbn[ 0 ] * anisotropyV.x + tbn[ 1 ] * anisotropyV.y;
	material.anisotropyB = tbn[ 1 ] * anisotropyV.x - tbn[ 0 ] * anisotropyV.y;
#endif`,rm=`struct PhysicalMaterial {
	vec3 diffuseColor;
	float roughness;
	vec3 specularColor;
	float specularF90;
	float dispersion;
	#ifdef USE_CLEARCOAT
		float clearcoat;
		float clearcoatRoughness;
		vec3 clearcoatF0;
		float clearcoatF90;
	#endif
	#ifdef USE_IRIDESCENCE
		float iridescence;
		float iridescenceIOR;
		float iridescenceThickness;
		vec3 iridescenceFresnel;
		vec3 iridescenceF0;
	#endif
	#ifdef USE_SHEEN
		vec3 sheenColor;
		float sheenRoughness;
	#endif
	#ifdef IOR
		float ior;
	#endif
	#ifdef USE_TRANSMISSION
		float transmission;
		float transmissionAlpha;
		float thickness;
		float attenuationDistance;
		vec3 attenuationColor;
	#endif
	#ifdef USE_ANISOTROPY
		float anisotropy;
		float alphaT;
		vec3 anisotropyT;
		vec3 anisotropyB;
	#endif
};
vec3 clearcoatSpecularDirect = vec3( 0.0 );
vec3 clearcoatSpecularIndirect = vec3( 0.0 );
vec3 sheenSpecularDirect = vec3( 0.0 );
vec3 sheenSpecularIndirect = vec3(0.0 );
vec3 Schlick_to_F0( const in vec3 f, const in float f90, const in float dotVH ) {
    float x = clamp( 1.0 - dotVH, 0.0, 1.0 );
    float x2 = x * x;
    float x5 = clamp( x * x2 * x2, 0.0, 0.9999 );
    return ( f - vec3( f90 ) * x5 ) / ( 1.0 - x5 );
}
float V_GGX_SmithCorrelated( const in float alpha, const in float dotNL, const in float dotNV ) {
	float a2 = pow2( alpha );
	float gv = dotNL * sqrt( a2 + ( 1.0 - a2 ) * pow2( dotNV ) );
	float gl = dotNV * sqrt( a2 + ( 1.0 - a2 ) * pow2( dotNL ) );
	return 0.5 / max( gv + gl, EPSILON );
}
float D_GGX( const in float alpha, const in float dotNH ) {
	float a2 = pow2( alpha );
	float denom = pow2( dotNH ) * ( a2 - 1.0 ) + 1.0;
	return RECIPROCAL_PI * a2 / pow2( denom );
}
#ifdef USE_ANISOTROPY
	float V_GGX_SmithCorrelated_Anisotropic( const in float alphaT, const in float alphaB, const in float dotTV, const in float dotBV, const in float dotTL, const in float dotBL, const in float dotNV, const in float dotNL ) {
		float gv = dotNL * length( vec3( alphaT * dotTV, alphaB * dotBV, dotNV ) );
		float gl = dotNV * length( vec3( alphaT * dotTL, alphaB * dotBL, dotNL ) );
		float v = 0.5 / ( gv + gl );
		return saturate(v);
	}
	float D_GGX_Anisotropic( const in float alphaT, const in float alphaB, const in float dotNH, const in float dotTH, const in float dotBH ) {
		float a2 = alphaT * alphaB;
		highp vec3 v = vec3( alphaB * dotTH, alphaT * dotBH, a2 * dotNH );
		highp float v2 = dot( v, v );
		float w2 = a2 / v2;
		return RECIPROCAL_PI * a2 * pow2 ( w2 );
	}
#endif
#ifdef USE_CLEARCOAT
	vec3 BRDF_GGX_Clearcoat( const in vec3 lightDir, const in vec3 viewDir, const in vec3 normal, const in PhysicalMaterial material) {
		vec3 f0 = material.clearcoatF0;
		float f90 = material.clearcoatF90;
		float roughness = material.clearcoatRoughness;
		float alpha = pow2( roughness );
		vec3 halfDir = normalize( lightDir + viewDir );
		float dotNL = saturate( dot( normal, lightDir ) );
		float dotNV = saturate( dot( normal, viewDir ) );
		float dotNH = saturate( dot( normal, halfDir ) );
		float dotVH = saturate( dot( viewDir, halfDir ) );
		vec3 F = F_Schlick( f0, f90, dotVH );
		float V = V_GGX_SmithCorrelated( alpha, dotNL, dotNV );
		float D = D_GGX( alpha, dotNH );
		return F * ( V * D );
	}
#endif
vec3 BRDF_GGX( const in vec3 lightDir, const in vec3 viewDir, const in vec3 normal, const in PhysicalMaterial material ) {
	vec3 f0 = material.specularColor;
	float f90 = material.specularF90;
	float roughness = material.roughness;
	float alpha = pow2( roughness );
	vec3 halfDir = normalize( lightDir + viewDir );
	float dotNL = saturate( dot( normal, lightDir ) );
	float dotNV = saturate( dot( normal, viewDir ) );
	float dotNH = saturate( dot( normal, halfDir ) );
	float dotVH = saturate( dot( viewDir, halfDir ) );
	vec3 F = F_Schlick( f0, f90, dotVH );
	#ifdef USE_IRIDESCENCE
		F = mix( F, material.iridescenceFresnel, material.iridescence );
	#endif
	#ifdef USE_ANISOTROPY
		float dotTL = dot( material.anisotropyT, lightDir );
		float dotTV = dot( material.anisotropyT, viewDir );
		float dotTH = dot( material.anisotropyT, halfDir );
		float dotBL = dot( material.anisotropyB, lightDir );
		float dotBV = dot( material.anisotropyB, viewDir );
		float dotBH = dot( material.anisotropyB, halfDir );
		float V = V_GGX_SmithCorrelated_Anisotropic( material.alphaT, alpha, dotTV, dotBV, dotTL, dotBL, dotNV, dotNL );
		float D = D_GGX_Anisotropic( material.alphaT, alpha, dotNH, dotTH, dotBH );
	#else
		float V = V_GGX_SmithCorrelated( alpha, dotNL, dotNV );
		float D = D_GGX( alpha, dotNH );
	#endif
	return F * ( V * D );
}
vec2 LTC_Uv( const in vec3 N, const in vec3 V, const in float roughness ) {
	const float LUT_SIZE = 64.0;
	const float LUT_SCALE = ( LUT_SIZE - 1.0 ) / LUT_SIZE;
	const float LUT_BIAS = 0.5 / LUT_SIZE;
	float dotNV = saturate( dot( N, V ) );
	vec2 uv = vec2( roughness, sqrt( 1.0 - dotNV ) );
	uv = uv * LUT_SCALE + LUT_BIAS;
	return uv;
}
float LTC_ClippedSphereFormFactor( const in vec3 f ) {
	float l = length( f );
	return max( ( l * l + f.z ) / ( l + 1.0 ), 0.0 );
}
vec3 LTC_EdgeVectorFormFactor( const in vec3 v1, const in vec3 v2 ) {
	float x = dot( v1, v2 );
	float y = abs( x );
	float a = 0.8543985 + ( 0.4965155 + 0.0145206 * y ) * y;
	float b = 3.4175940 + ( 4.1616724 + y ) * y;
	float v = a / b;
	float theta_sintheta = ( x > 0.0 ) ? v : 0.5 * inversesqrt( max( 1.0 - x * x, 1e-7 ) ) - v;
	return cross( v1, v2 ) * theta_sintheta;
}
vec3 LTC_Evaluate( const in vec3 N, const in vec3 V, const in vec3 P, const in mat3 mInv, const in vec3 rectCoords[ 4 ] ) {
	vec3 v1 = rectCoords[ 1 ] - rectCoords[ 0 ];
	vec3 v2 = rectCoords[ 3 ] - rectCoords[ 0 ];
	vec3 lightNormal = cross( v1, v2 );
	if( dot( lightNormal, P - rectCoords[ 0 ] ) < 0.0 ) return vec3( 0.0 );
	vec3 T1, T2;
	T1 = normalize( V - N * dot( V, N ) );
	T2 = - cross( N, T1 );
	mat3 mat = mInv * transposeMat3( mat3( T1, T2, N ) );
	vec3 coords[ 4 ];
	coords[ 0 ] = mat * ( rectCoords[ 0 ] - P );
	coords[ 1 ] = mat * ( rectCoords[ 1 ] - P );
	coords[ 2 ] = mat * ( rectCoords[ 2 ] - P );
	coords[ 3 ] = mat * ( rectCoords[ 3 ] - P );
	coords[ 0 ] = normalize( coords[ 0 ] );
	coords[ 1 ] = normalize( coords[ 1 ] );
	coords[ 2 ] = normalize( coords[ 2 ] );
	coords[ 3 ] = normalize( coords[ 3 ] );
	vec3 vectorFormFactor = vec3( 0.0 );
	vectorFormFactor += LTC_EdgeVectorFormFactor( coords[ 0 ], coords[ 1 ] );
	vectorFormFactor += LTC_EdgeVectorFormFactor( coords[ 1 ], coords[ 2 ] );
	vectorFormFactor += LTC_EdgeVectorFormFactor( coords[ 2 ], coords[ 3 ] );
	vectorFormFactor += LTC_EdgeVectorFormFactor( coords[ 3 ], coords[ 0 ] );
	float result = LTC_ClippedSphereFormFactor( vectorFormFactor );
	return vec3( result );
}
#if defined( USE_SHEEN )
float D_Charlie( float roughness, float dotNH ) {
	float alpha = pow2( roughness );
	float invAlpha = 1.0 / alpha;
	float cos2h = dotNH * dotNH;
	float sin2h = max( 1.0 - cos2h, 0.0078125 );
	return ( 2.0 + invAlpha ) * pow( sin2h, invAlpha * 0.5 ) / ( 2.0 * PI );
}
float V_Neubelt( float dotNV, float dotNL ) {
	return saturate( 1.0 / ( 4.0 * ( dotNL + dotNV - dotNL * dotNV ) ) );
}
vec3 BRDF_Sheen( const in vec3 lightDir, const in vec3 viewDir, const in vec3 normal, vec3 sheenColor, const in float sheenRoughness ) {
	vec3 halfDir = normalize( lightDir + viewDir );
	float dotNL = saturate( dot( normal, lightDir ) );
	float dotNV = saturate( dot( normal, viewDir ) );
	float dotNH = saturate( dot( normal, halfDir ) );
	float D = D_Charlie( sheenRoughness, dotNH );
	float V = V_Neubelt( dotNV, dotNL );
	return sheenColor * ( D * V );
}
#endif
float IBLSheenBRDF( const in vec3 normal, const in vec3 viewDir, const in float roughness ) {
	float dotNV = saturate( dot( normal, viewDir ) );
	float r2 = roughness * roughness;
	float a = roughness < 0.25 ? -339.2 * r2 + 161.4 * roughness - 25.9 : -8.48 * r2 + 14.3 * roughness - 9.95;
	float b = roughness < 0.25 ? 44.0 * r2 - 23.7 * roughness + 3.26 : 1.97 * r2 - 3.27 * roughness + 0.72;
	float DG = exp( a * dotNV + b ) + ( roughness < 0.25 ? 0.0 : 0.1 * ( roughness - 0.25 ) );
	return saturate( DG * RECIPROCAL_PI );
}
vec2 DFGApprox( const in vec3 normal, const in vec3 viewDir, const in float roughness ) {
	float dotNV = saturate( dot( normal, viewDir ) );
	const vec4 c0 = vec4( - 1, - 0.0275, - 0.572, 0.022 );
	const vec4 c1 = vec4( 1, 0.0425, 1.04, - 0.04 );
	vec4 r = roughness * c0 + c1;
	float a004 = min( r.x * r.x, exp2( - 9.28 * dotNV ) ) * r.x + r.y;
	vec2 fab = vec2( - 1.04, 1.04 ) * a004 + r.zw;
	return fab;
}
vec3 EnvironmentBRDF( const in vec3 normal, const in vec3 viewDir, const in vec3 specularColor, const in float specularF90, const in float roughness ) {
	vec2 fab = DFGApprox( normal, viewDir, roughness );
	return specularColor * fab.x + specularF90 * fab.y;
}
#ifdef USE_IRIDESCENCE
void computeMultiscatteringIridescence( const in vec3 normal, const in vec3 viewDir, const in vec3 specularColor, const in float specularF90, const in float iridescence, const in vec3 iridescenceF0, const in float roughness, inout vec3 singleScatter, inout vec3 multiScatter ) {
#else
void computeMultiscattering( const in vec3 normal, const in vec3 viewDir, const in vec3 specularColor, const in float specularF90, const in float roughness, inout vec3 singleScatter, inout vec3 multiScatter ) {
#endif
	vec2 fab = DFGApprox( normal, viewDir, roughness );
	#ifdef USE_IRIDESCENCE
		vec3 Fr = mix( specularColor, iridescenceF0, iridescence );
	#else
		vec3 Fr = specularColor;
	#endif
	vec3 FssEss = Fr * fab.x + specularF90 * fab.y;
	float Ess = fab.x + fab.y;
	float Ems = 1.0 - Ess;
	vec3 Favg = Fr + ( 1.0 - Fr ) * 0.047619;	vec3 Fms = FssEss * Favg / ( 1.0 - Ems * Favg );
	singleScatter += FssEss;
	multiScatter += Fms * Ems;
}
#if NUM_RECT_AREA_LIGHTS > 0
	void RE_Direct_RectArea_Physical( const in RectAreaLight rectAreaLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in PhysicalMaterial material, inout ReflectedLight reflectedLight ) {
		vec3 normal = geometryNormal;
		vec3 viewDir = geometryViewDir;
		vec3 position = geometryPosition;
		vec3 lightPos = rectAreaLight.position;
		vec3 halfWidth = rectAreaLight.halfWidth;
		vec3 halfHeight = rectAreaLight.halfHeight;
		vec3 lightColor = rectAreaLight.color;
		float roughness = material.roughness;
		vec3 rectCoords[ 4 ];
		rectCoords[ 0 ] = lightPos + halfWidth - halfHeight;		rectCoords[ 1 ] = lightPos - halfWidth - halfHeight;
		rectCoords[ 2 ] = lightPos - halfWidth + halfHeight;
		rectCoords[ 3 ] = lightPos + halfWidth + halfHeight;
		vec2 uv = LTC_Uv( normal, viewDir, roughness );
		vec4 t1 = texture2D( ltc_1, uv );
		vec4 t2 = texture2D( ltc_2, uv );
		mat3 mInv = mat3(
			vec3( t1.x, 0, t1.y ),
			vec3(    0, 1,    0 ),
			vec3( t1.z, 0, t1.w )
		);
		vec3 fresnel = ( material.specularColor * t2.x + ( vec3( 1.0 ) - material.specularColor ) * t2.y );
		reflectedLight.directSpecular += lightColor * fresnel * LTC_Evaluate( normal, viewDir, position, mInv, rectCoords );
		reflectedLight.directDiffuse += lightColor * material.diffuseColor * LTC_Evaluate( normal, viewDir, position, mat3( 1.0 ), rectCoords );
	}
#endif
void RE_Direct_Physical( const in IncidentLight directLight, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in PhysicalMaterial material, inout ReflectedLight reflectedLight ) {
	float dotNL = saturate( dot( geometryNormal, directLight.direction ) );
	vec3 irradiance = dotNL * directLight.color;
	#ifdef USE_CLEARCOAT
		float dotNLcc = saturate( dot( geometryClearcoatNormal, directLight.direction ) );
		vec3 ccIrradiance = dotNLcc * directLight.color;
		clearcoatSpecularDirect += ccIrradiance * BRDF_GGX_Clearcoat( directLight.direction, geometryViewDir, geometryClearcoatNormal, material );
	#endif
	#ifdef USE_SHEEN
		sheenSpecularDirect += irradiance * BRDF_Sheen( directLight.direction, geometryViewDir, geometryNormal, material.sheenColor, material.sheenRoughness );
	#endif
	reflectedLight.directSpecular += irradiance * BRDF_GGX( directLight.direction, geometryViewDir, geometryNormal, material );
	reflectedLight.directDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
void RE_IndirectDiffuse_Physical( const in vec3 irradiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in PhysicalMaterial material, inout ReflectedLight reflectedLight ) {
	reflectedLight.indirectDiffuse += irradiance * BRDF_Lambert( material.diffuseColor );
}
void RE_IndirectSpecular_Physical( const in vec3 radiance, const in vec3 irradiance, const in vec3 clearcoatRadiance, const in vec3 geometryPosition, const in vec3 geometryNormal, const in vec3 geometryViewDir, const in vec3 geometryClearcoatNormal, const in PhysicalMaterial material, inout ReflectedLight reflectedLight) {
	#ifdef USE_CLEARCOAT
		clearcoatSpecularIndirect += clearcoatRadiance * EnvironmentBRDF( geometryClearcoatNormal, geometryViewDir, material.clearcoatF0, material.clearcoatF90, material.clearcoatRoughness );
	#endif
	#ifdef USE_SHEEN
		sheenSpecularIndirect += irradiance * material.sheenColor * IBLSheenBRDF( geometryNormal, geometryViewDir, material.sheenRoughness );
	#endif
	vec3 singleScattering = vec3( 0.0 );
	vec3 multiScattering = vec3( 0.0 );
	vec3 cosineWeightedIrradiance = irradiance * RECIPROCAL_PI;
	#ifdef USE_IRIDESCENCE
		computeMultiscatteringIridescence( geometryNormal, geometryViewDir, material.specularColor, material.specularF90, material.iridescence, material.iridescenceFresnel, material.roughness, singleScattering, multiScattering );
	#else
		computeMultiscattering( geometryNormal, geometryViewDir, material.specularColor, material.specularF90, material.roughness, singleScattering, multiScattering );
	#endif
	vec3 totalScattering = singleScattering + multiScattering;
	vec3 diffuse = material.diffuseColor * ( 1.0 - max( max( totalScattering.r, totalScattering.g ), totalScattering.b ) );
	reflectedLight.indirectSpecular += radiance * singleScattering;
	reflectedLight.indirectSpecular += multiScattering * cosineWeightedIrradiance;
	reflectedLight.indirectDiffuse += diffuse * cosineWeightedIrradiance;
}
#define RE_Direct				RE_Direct_Physical
#define RE_Direct_RectArea		RE_Direct_RectArea_Physical
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Physical
#define RE_IndirectSpecular		RE_IndirectSpecular_Physical
float computeSpecularOcclusion( const in float dotNV, const in float ambientOcclusion, const in float roughness ) {
	return saturate( pow( dotNV + ambientOcclusion, exp2( - 16.0 * roughness - 1.0 ) ) - 1.0 + ambientOcclusion );
}`,om=`
vec3 geometryPosition = - vViewPosition;
vec3 geometryNormal = normal;
vec3 geometryViewDir = ( isOrthographic ) ? vec3( 0, 0, 1 ) : normalize( vViewPosition );
vec3 geometryClearcoatNormal = vec3( 0.0 );
#ifdef USE_CLEARCOAT
	geometryClearcoatNormal = clearcoatNormal;
#endif
#ifdef USE_IRIDESCENCE
	float dotNVi = saturate( dot( normal, geometryViewDir ) );
	if ( material.iridescenceThickness == 0.0 ) {
		material.iridescence = 0.0;
	} else {
		material.iridescence = saturate( material.iridescence );
	}
	if ( material.iridescence > 0.0 ) {
		material.iridescenceFresnel = evalIridescence( 1.0, material.iridescenceIOR, dotNVi, material.iridescenceThickness, material.specularColor );
		material.iridescenceF0 = Schlick_to_F0( material.iridescenceFresnel, 1.0, dotNVi );
	}
#endif
IncidentLight directLight;
#if ( NUM_POINT_LIGHTS > 0 ) && defined( RE_Direct )
	PointLight pointLight;
	#if defined( USE_SHADOWMAP ) && NUM_POINT_LIGHT_SHADOWS > 0
	PointLightShadow pointLightShadow;
	#endif
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_POINT_LIGHTS; i ++ ) {
		pointLight = pointLights[ i ];
		getPointLightInfo( pointLight, geometryPosition, directLight );
		#if defined( USE_SHADOWMAP ) && ( UNROLLED_LOOP_INDEX < NUM_POINT_LIGHT_SHADOWS )
		pointLightShadow = pointLightShadows[ i ];
		directLight.color *= ( directLight.visible && receiveShadow ) ? getPointShadow( pointShadowMap[ i ], pointLightShadow.shadowMapSize, pointLightShadow.shadowIntensity, pointLightShadow.shadowBias, pointLightShadow.shadowRadius, vPointShadowCoord[ i ], pointLightShadow.shadowCameraNear, pointLightShadow.shadowCameraFar ) : 1.0;
		#endif
		RE_Direct( directLight, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
	}
	#pragma unroll_loop_end
#endif
#if ( NUM_SPOT_LIGHTS > 0 ) && defined( RE_Direct )
	SpotLight spotLight;
	vec4 spotColor;
	vec3 spotLightCoord;
	bool inSpotLightMap;
	#if defined( USE_SHADOWMAP ) && NUM_SPOT_LIGHT_SHADOWS > 0
	SpotLightShadow spotLightShadow;
	#endif
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_SPOT_LIGHTS; i ++ ) {
		spotLight = spotLights[ i ];
		getSpotLightInfo( spotLight, geometryPosition, directLight );
		#if ( UNROLLED_LOOP_INDEX < NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS )
		#define SPOT_LIGHT_MAP_INDEX UNROLLED_LOOP_INDEX
		#elif ( UNROLLED_LOOP_INDEX < NUM_SPOT_LIGHT_SHADOWS )
		#define SPOT_LIGHT_MAP_INDEX NUM_SPOT_LIGHT_MAPS
		#else
		#define SPOT_LIGHT_MAP_INDEX ( UNROLLED_LOOP_INDEX - NUM_SPOT_LIGHT_SHADOWS + NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS )
		#endif
		#if ( SPOT_LIGHT_MAP_INDEX < NUM_SPOT_LIGHT_MAPS )
			spotLightCoord = vSpotLightCoord[ i ].xyz / vSpotLightCoord[ i ].w;
			inSpotLightMap = all( lessThan( abs( spotLightCoord * 2. - 1. ), vec3( 1.0 ) ) );
			spotColor = texture2D( spotLightMap[ SPOT_LIGHT_MAP_INDEX ], spotLightCoord.xy );
			directLight.color = inSpotLightMap ? directLight.color * spotColor.rgb : directLight.color;
		#endif
		#undef SPOT_LIGHT_MAP_INDEX
		#if defined( USE_SHADOWMAP ) && ( UNROLLED_LOOP_INDEX < NUM_SPOT_LIGHT_SHADOWS )
		spotLightShadow = spotLightShadows[ i ];
		directLight.color *= ( directLight.visible && receiveShadow ) ? getShadow( spotShadowMap[ i ], spotLightShadow.shadowMapSize, spotLightShadow.shadowIntensity, spotLightShadow.shadowBias, spotLightShadow.shadowRadius, vSpotLightCoord[ i ] ) : 1.0;
		#endif
		RE_Direct( directLight, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
	}
	#pragma unroll_loop_end
#endif
#if ( NUM_DIR_LIGHTS > 0 ) && defined( RE_Direct )
	DirectionalLight directionalLight;
	#if defined( USE_SHADOWMAP ) && NUM_DIR_LIGHT_SHADOWS > 0
	DirectionalLightShadow directionalLightShadow;
	#endif
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_DIR_LIGHTS; i ++ ) {
		directionalLight = directionalLights[ i ];
		getDirectionalLightInfo( directionalLight, directLight );
		#if defined( USE_SHADOWMAP ) && ( UNROLLED_LOOP_INDEX < NUM_DIR_LIGHT_SHADOWS )
		directionalLightShadow = directionalLightShadows[ i ];
		directLight.color *= ( directLight.visible && receiveShadow ) ? getShadow( directionalShadowMap[ i ], directionalLightShadow.shadowMapSize, directionalLightShadow.shadowIntensity, directionalLightShadow.shadowBias, directionalLightShadow.shadowRadius, vDirectionalShadowCoord[ i ] ) : 1.0;
		#endif
		RE_Direct( directLight, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
	}
	#pragma unroll_loop_end
#endif
#if ( NUM_RECT_AREA_LIGHTS > 0 ) && defined( RE_Direct_RectArea )
	RectAreaLight rectAreaLight;
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_RECT_AREA_LIGHTS; i ++ ) {
		rectAreaLight = rectAreaLights[ i ];
		RE_Direct_RectArea( rectAreaLight, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
	}
	#pragma unroll_loop_end
#endif
#if defined( RE_IndirectDiffuse )
	vec3 iblIrradiance = vec3( 0.0 );
	vec3 irradiance = getAmbientLightIrradiance( ambientLightColor );
	#if defined( USE_LIGHT_PROBES )
		irradiance += getLightProbeIrradiance( lightProbe, geometryNormal );
	#endif
	#if ( NUM_HEMI_LIGHTS > 0 )
		#pragma unroll_loop_start
		for ( int i = 0; i < NUM_HEMI_LIGHTS; i ++ ) {
			irradiance += getHemisphereLightIrradiance( hemisphereLights[ i ], geometryNormal );
		}
		#pragma unroll_loop_end
	#endif
#endif
#if defined( RE_IndirectSpecular )
	vec3 radiance = vec3( 0.0 );
	vec3 clearcoatRadiance = vec3( 0.0 );
#endif`,am=`#if defined( RE_IndirectDiffuse )
	#ifdef USE_LIGHTMAP
		vec4 lightMapTexel = texture2D( lightMap, vLightMapUv );
		vec3 lightMapIrradiance = lightMapTexel.rgb * lightMapIntensity;
		irradiance += lightMapIrradiance;
	#endif
	#if defined( USE_ENVMAP ) && defined( STANDARD ) && defined( ENVMAP_TYPE_CUBE_UV )
		iblIrradiance += getIBLIrradiance( geometryNormal );
	#endif
#endif
#if defined( USE_ENVMAP ) && defined( RE_IndirectSpecular )
	#ifdef USE_ANISOTROPY
		radiance += getIBLAnisotropyRadiance( geometryViewDir, geometryNormal, material.roughness, material.anisotropyB, material.anisotropy );
	#else
		radiance += getIBLRadiance( geometryViewDir, geometryNormal, material.roughness );
	#endif
	#ifdef USE_CLEARCOAT
		clearcoatRadiance += getIBLRadiance( geometryViewDir, geometryClearcoatNormal, material.clearcoatRoughness );
	#endif
#endif`,lm=`#if defined( RE_IndirectDiffuse )
	RE_IndirectDiffuse( irradiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif
#if defined( RE_IndirectSpecular )
	RE_IndirectSpecular( radiance, iblIrradiance, clearcoatRadiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif`,cm=`#if defined( USE_LOGDEPTHBUF )
	gl_FragDepth = vIsPerspective == 0.0 ? gl_FragCoord.z : log2( vFragDepth ) * logDepthBufFC * 0.5;
#endif`,hm=`#if defined( USE_LOGDEPTHBUF )
	uniform float logDepthBufFC;
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,um=`#ifdef USE_LOGDEPTHBUF
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,dm=`#ifdef USE_LOGDEPTHBUF
	vFragDepth = 1.0 + gl_Position.w;
	vIsPerspective = float( isPerspectiveMatrix( projectionMatrix ) );
#endif`,fm=`#ifdef USE_MAP
	vec4 sampledDiffuseColor = texture2D( map, vMapUv );
	#ifdef DECODE_VIDEO_TEXTURE
		sampledDiffuseColor = vec4( mix( pow( sampledDiffuseColor.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), sampledDiffuseColor.rgb * 0.0773993808, vec3( lessThanEqual( sampledDiffuseColor.rgb, vec3( 0.04045 ) ) ) ), sampledDiffuseColor.w );
	
	#endif
	diffuseColor *= sampledDiffuseColor;
#endif`,pm=`#ifdef USE_MAP
	uniform sampler2D map;
#endif`,mm=`#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
	#if defined( USE_POINTS_UV )
		vec2 uv = vUv;
	#else
		vec2 uv = ( uvTransform * vec3( gl_PointCoord.x, 1.0 - gl_PointCoord.y, 1 ) ).xy;
	#endif
#endif
#ifdef USE_MAP
	diffuseColor *= texture2D( map, uv );
#endif
#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, uv ).g;
#endif`,gm=`#if defined( USE_POINTS_UV )
	varying vec2 vUv;
#else
	#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
		uniform mat3 uvTransform;
	#endif
#endif
#ifdef USE_MAP
	uniform sampler2D map;
#endif
#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,xm=`float metalnessFactor = metalness;
#ifdef USE_METALNESSMAP
	vec4 texelMetalness = texture2D( metalnessMap, vMetalnessMapUv );
	metalnessFactor *= texelMetalness.b;
#endif`,vm=`#ifdef USE_METALNESSMAP
	uniform sampler2D metalnessMap;
#endif`,_m=`#ifdef USE_INSTANCING_MORPH
	float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	float morphTargetBaseInfluence = texelFetch( morphTexture, ivec2( 0, gl_InstanceID ), 0 ).r;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		morphTargetInfluences[i] =  texelFetch( morphTexture, ivec2( i + 1, gl_InstanceID ), 0 ).r;
	}
#endif`,ym=`#if defined( USE_MORPHCOLORS )
	vColor *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		#if defined( USE_COLOR_ALPHA )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ) * morphTargetInfluences[ i ];
		#elif defined( USE_COLOR )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ).rgb * morphTargetInfluences[ i ];
		#endif
	}
#endif`,Mm=`#ifdef USE_MORPHNORMALS
	objectNormal *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) objectNormal += getMorph( gl_VertexID, i, 1 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,Sm=`#ifdef USE_MORPHTARGETS
	#ifndef USE_INSTANCING_MORPH
		uniform float morphTargetBaseInfluence;
		uniform float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	#endif
	uniform sampler2DArray morphTargetsTexture;
	uniform ivec2 morphTargetsTextureSize;
	vec4 getMorph( const in int vertexIndex, const in int morphTargetIndex, const in int offset ) {
		int texelIndex = vertexIndex * MORPHTARGETS_TEXTURE_STRIDE + offset;
		int y = texelIndex / morphTargetsTextureSize.x;
		int x = texelIndex - y * morphTargetsTextureSize.x;
		ivec3 morphUV = ivec3( x, y, morphTargetIndex );
		return texelFetch( morphTargetsTexture, morphUV, 0 );
	}
#endif`,bm=`#ifdef USE_MORPHTARGETS
	transformed *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) transformed += getMorph( gl_VertexID, i, 0 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,wm=`float faceDirection = gl_FrontFacing ? 1.0 : - 1.0;
#ifdef FLAT_SHADED
	vec3 fdx = dFdx( vViewPosition );
	vec3 fdy = dFdy( vViewPosition );
	vec3 normal = normalize( cross( fdx, fdy ) );
#else
	vec3 normal = normalize( vNormal );
	#ifdef DOUBLE_SIDED
		normal *= faceDirection;
	#endif
#endif
#if defined( USE_NORMALMAP_TANGENTSPACE ) || defined( USE_CLEARCOAT_NORMALMAP ) || defined( USE_ANISOTROPY )
	#ifdef USE_TANGENT
		mat3 tbn = mat3( normalize( vTangent ), normalize( vBitangent ), normal );
	#else
		mat3 tbn = getTangentFrame( - vViewPosition, normal,
		#if defined( USE_NORMALMAP )
			vNormalMapUv
		#elif defined( USE_CLEARCOAT_NORMALMAP )
			vClearcoatNormalMapUv
		#else
			vUv
		#endif
		);
	#endif
	#if defined( DOUBLE_SIDED ) && ! defined( FLAT_SHADED )
		tbn[0] *= faceDirection;
		tbn[1] *= faceDirection;
	#endif
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	#ifdef USE_TANGENT
		mat3 tbn2 = mat3( normalize( vTangent ), normalize( vBitangent ), normal );
	#else
		mat3 tbn2 = getTangentFrame( - vViewPosition, normal, vClearcoatNormalMapUv );
	#endif
	#if defined( DOUBLE_SIDED ) && ! defined( FLAT_SHADED )
		tbn2[0] *= faceDirection;
		tbn2[1] *= faceDirection;
	#endif
#endif
vec3 nonPerturbedNormal = normal;`,Am=`#ifdef USE_NORMALMAP_OBJECTSPACE
	normal = texture2D( normalMap, vNormalMapUv ).xyz * 2.0 - 1.0;
	#ifdef FLIP_SIDED
		normal = - normal;
	#endif
	#ifdef DOUBLE_SIDED
		normal = normal * faceDirection;
	#endif
	normal = normalize( normalMatrix * normal );
#elif defined( USE_NORMALMAP_TANGENTSPACE )
	vec3 mapN = texture2D( normalMap, vNormalMapUv ).xyz * 2.0 - 1.0;
	mapN.xy *= normalScale;
	normal = normalize( tbn * mapN );
#elif defined( USE_BUMPMAP )
	normal = perturbNormalArb( - vViewPosition, normal, dHdxy_fwd(), faceDirection );
#endif`,Em=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,Tm=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,Cm=`#ifndef FLAT_SHADED
	vNormal = normalize( transformedNormal );
	#ifdef USE_TANGENT
		vTangent = normalize( transformedTangent );
		vBitangent = normalize( cross( vNormal, vTangent ) * tangent.w );
	#endif
#endif`,Rm=`#ifdef USE_NORMALMAP
	uniform sampler2D normalMap;
	uniform vec2 normalScale;
#endif
#ifdef USE_NORMALMAP_OBJECTSPACE
	uniform mat3 normalMatrix;
#endif
#if ! defined ( USE_TANGENT ) && ( defined ( USE_NORMALMAP_TANGENTSPACE ) || defined ( USE_CLEARCOAT_NORMALMAP ) || defined( USE_ANISOTROPY ) )
	mat3 getTangentFrame( vec3 eye_pos, vec3 surf_norm, vec2 uv ) {
		vec3 q0 = dFdx( eye_pos.xyz );
		vec3 q1 = dFdy( eye_pos.xyz );
		vec2 st0 = dFdx( uv.st );
		vec2 st1 = dFdy( uv.st );
		vec3 N = surf_norm;
		vec3 q1perp = cross( q1, N );
		vec3 q0perp = cross( N, q0 );
		vec3 T = q1perp * st0.x + q0perp * st1.x;
		vec3 B = q1perp * st0.y + q0perp * st1.y;
		float det = max( dot( T, T ), dot( B, B ) );
		float scale = ( det == 0.0 ) ? 0.0 : inversesqrt( det );
		return mat3( T * scale, B * scale, N );
	}
#endif`,Pm=`#ifdef USE_CLEARCOAT
	vec3 clearcoatNormal = nonPerturbedNormal;
#endif`,Im=`#ifdef USE_CLEARCOAT_NORMALMAP
	vec3 clearcoatMapN = texture2D( clearcoatNormalMap, vClearcoatNormalMapUv ).xyz * 2.0 - 1.0;
	clearcoatMapN.xy *= clearcoatNormalScale;
	clearcoatNormal = normalize( tbn2 * clearcoatMapN );
#endif`,Lm=`#ifdef USE_CLEARCOATMAP
	uniform sampler2D clearcoatMap;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform sampler2D clearcoatNormalMap;
	uniform vec2 clearcoatNormalScale;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform sampler2D clearcoatRoughnessMap;
#endif`,Dm=`#ifdef USE_IRIDESCENCEMAP
	uniform sampler2D iridescenceMap;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform sampler2D iridescenceThicknessMap;
#endif`,Um=`#ifdef OPAQUE
diffuseColor.a = 1.0;
#endif
#ifdef USE_TRANSMISSION
diffuseColor.a *= material.transmissionAlpha;
#endif
gl_FragColor = vec4( outgoingLight, diffuseColor.a );`,Nm=`vec3 packNormalToRGB( const in vec3 normal ) {
	return normalize( normal ) * 0.5 + 0.5;
}
vec3 unpackRGBToNormal( const in vec3 rgb ) {
	return 2.0 * rgb.xyz - 1.0;
}
const float PackUpscale = 256. / 255.;const float UnpackDownscale = 255. / 256.;const float ShiftRight8 = 1. / 256.;
const float Inv255 = 1. / 255.;
const vec4 PackFactors = vec4( 1.0, 256.0, 256.0 * 256.0, 256.0 * 256.0 * 256.0 );
const vec2 UnpackFactors2 = vec2( UnpackDownscale, 1.0 / PackFactors.g );
const vec3 UnpackFactors3 = vec3( UnpackDownscale / PackFactors.rg, 1.0 / PackFactors.b );
const vec4 UnpackFactors4 = vec4( UnpackDownscale / PackFactors.rgb, 1.0 / PackFactors.a );
vec4 packDepthToRGBA( const in float v ) {
	if( v <= 0.0 )
		return vec4( 0., 0., 0., 0. );
	if( v >= 1.0 )
		return vec4( 1., 1., 1., 1. );
	float vuf;
	float af = modf( v * PackFactors.a, vuf );
	float bf = modf( vuf * ShiftRight8, vuf );
	float gf = modf( vuf * ShiftRight8, vuf );
	return vec4( vuf * Inv255, gf * PackUpscale, bf * PackUpscale, af );
}
vec3 packDepthToRGB( const in float v ) {
	if( v <= 0.0 )
		return vec3( 0., 0., 0. );
	if( v >= 1.0 )
		return vec3( 1., 1., 1. );
	float vuf;
	float bf = modf( v * PackFactors.b, vuf );
	float gf = modf( vuf * ShiftRight8, vuf );
	return vec3( vuf * Inv255, gf * PackUpscale, bf );
}
vec2 packDepthToRG( const in float v ) {
	if( v <= 0.0 )
		return vec2( 0., 0. );
	if( v >= 1.0 )
		return vec2( 1., 1. );
	float vuf;
	float gf = modf( v * 256., vuf );
	return vec2( vuf * Inv255, gf );
}
float unpackRGBAToDepth( const in vec4 v ) {
	return dot( v, UnpackFactors4 );
}
float unpackRGBToDepth( const in vec3 v ) {
	return dot( v, UnpackFactors3 );
}
float unpackRGToDepth( const in vec2 v ) {
	return v.r * UnpackFactors2.r + v.g * UnpackFactors2.g;
}
vec4 pack2HalfToRGBA( const in vec2 v ) {
	vec4 r = vec4( v.x, fract( v.x * 255.0 ), v.y, fract( v.y * 255.0 ) );
	return vec4( r.x - r.y / 255.0, r.y, r.z - r.w / 255.0, r.w );
}
vec2 unpackRGBATo2Half( const in vec4 v ) {
	return vec2( v.x + ( v.y / 255.0 ), v.z + ( v.w / 255.0 ) );
}
float viewZToOrthographicDepth( const in float viewZ, const in float near, const in float far ) {
	return ( viewZ + near ) / ( near - far );
}
float orthographicDepthToViewZ( const in float depth, const in float near, const in float far ) {
	return depth * ( near - far ) - near;
}
float viewZToPerspectiveDepth( const in float viewZ, const in float near, const in float far ) {
	return ( ( near + viewZ ) * far ) / ( ( far - near ) * viewZ );
}
float perspectiveDepthToViewZ( const in float depth, const in float near, const in float far ) {
	return ( near * far ) / ( ( far - near ) * depth - far );
}`,Om=`#ifdef PREMULTIPLIED_ALPHA
	gl_FragColor.rgb *= gl_FragColor.a;
#endif`,Fm=`vec4 mvPosition = vec4( transformed, 1.0 );
#ifdef USE_BATCHING
	mvPosition = batchingMatrix * mvPosition;
#endif
#ifdef USE_INSTANCING
	mvPosition = instanceMatrix * mvPosition;
#endif
mvPosition = modelViewMatrix * mvPosition;
gl_Position = projectionMatrix * mvPosition;`,Bm=`#ifdef DITHERING
	gl_FragColor.rgb = dithering( gl_FragColor.rgb );
#endif`,zm=`#ifdef DITHERING
	vec3 dithering( vec3 color ) {
		float grid_position = rand( gl_FragCoord.xy );
		vec3 dither_shift_RGB = vec3( 0.25 / 255.0, -0.25 / 255.0, 0.25 / 255.0 );
		dither_shift_RGB = mix( 2.0 * dither_shift_RGB, -2.0 * dither_shift_RGB, grid_position );
		return color + dither_shift_RGB;
	}
#endif`,km=`float roughnessFactor = roughness;
#ifdef USE_ROUGHNESSMAP
	vec4 texelRoughness = texture2D( roughnessMap, vRoughnessMapUv );
	roughnessFactor *= texelRoughness.g;
#endif`,Hm=`#ifdef USE_ROUGHNESSMAP
	uniform sampler2D roughnessMap;
#endif`,Vm=`#if NUM_SPOT_LIGHT_COORDS > 0
	varying vec4 vSpotLightCoord[ NUM_SPOT_LIGHT_COORDS ];
#endif
#if NUM_SPOT_LIGHT_MAPS > 0
	uniform sampler2D spotLightMap[ NUM_SPOT_LIGHT_MAPS ];
#endif
#ifdef USE_SHADOWMAP
	#if NUM_DIR_LIGHT_SHADOWS > 0
		uniform sampler2D directionalShadowMap[ NUM_DIR_LIGHT_SHADOWS ];
		varying vec4 vDirectionalShadowCoord[ NUM_DIR_LIGHT_SHADOWS ];
		struct DirectionalLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
		};
		uniform DirectionalLightShadow directionalLightShadows[ NUM_DIR_LIGHT_SHADOWS ];
	#endif
	#if NUM_SPOT_LIGHT_SHADOWS > 0
		uniform sampler2D spotShadowMap[ NUM_SPOT_LIGHT_SHADOWS ];
		struct SpotLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
		};
		uniform SpotLightShadow spotLightShadows[ NUM_SPOT_LIGHT_SHADOWS ];
	#endif
	#if NUM_POINT_LIGHT_SHADOWS > 0
		uniform sampler2D pointShadowMap[ NUM_POINT_LIGHT_SHADOWS ];
		varying vec4 vPointShadowCoord[ NUM_POINT_LIGHT_SHADOWS ];
		struct PointLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
			float shadowCameraNear;
			float shadowCameraFar;
		};
		uniform PointLightShadow pointLightShadows[ NUM_POINT_LIGHT_SHADOWS ];
	#endif
	float texture2DCompare( sampler2D depths, vec2 uv, float compare ) {
		return step( compare, unpackRGBAToDepth( texture2D( depths, uv ) ) );
	}
	vec2 texture2DDistribution( sampler2D shadow, vec2 uv ) {
		return unpackRGBATo2Half( texture2D( shadow, uv ) );
	}
	float VSMShadow (sampler2D shadow, vec2 uv, float compare ){
		float occlusion = 1.0;
		vec2 distribution = texture2DDistribution( shadow, uv );
		float hard_shadow = step( compare , distribution.x );
		if (hard_shadow != 1.0 ) {
			float distance = compare - distribution.x ;
			float variance = max( 0.00000, distribution.y * distribution.y );
			float softness_probability = variance / (variance + distance * distance );			softness_probability = clamp( ( softness_probability - 0.3 ) / ( 0.95 - 0.3 ), 0.0, 1.0 );			occlusion = clamp( max( hard_shadow, softness_probability ), 0.0, 1.0 );
		}
		return occlusion;
	}
	float getShadow( sampler2D shadowMap, vec2 shadowMapSize, float shadowIntensity, float shadowBias, float shadowRadius, vec4 shadowCoord ) {
		float shadow = 1.0;
		shadowCoord.xyz /= shadowCoord.w;
		shadowCoord.z += shadowBias;
		bool inFrustum = shadowCoord.x >= 0.0 && shadowCoord.x <= 1.0 && shadowCoord.y >= 0.0 && shadowCoord.y <= 1.0;
		bool frustumTest = inFrustum && shadowCoord.z <= 1.0;
		if ( frustumTest ) {
		#if defined( SHADOWMAP_TYPE_PCF )
			vec2 texelSize = vec2( 1.0 ) / shadowMapSize;
			float dx0 = - texelSize.x * shadowRadius;
			float dy0 = - texelSize.y * shadowRadius;
			float dx1 = + texelSize.x * shadowRadius;
			float dy1 = + texelSize.y * shadowRadius;
			float dx2 = dx0 / 2.0;
			float dy2 = dy0 / 2.0;
			float dx3 = dx1 / 2.0;
			float dy3 = dy1 / 2.0;
			shadow = (
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx0, dy0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( 0.0, dy0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx1, dy0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx2, dy2 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( 0.0, dy2 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx3, dy2 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx0, 0.0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx2, 0.0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy, shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx3, 0.0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx1, 0.0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx2, dy3 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( 0.0, dy3 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx3, dy3 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx0, dy1 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( 0.0, dy1 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, shadowCoord.xy + vec2( dx1, dy1 ), shadowCoord.z )
			) * ( 1.0 / 17.0 );
		#elif defined( SHADOWMAP_TYPE_PCF_SOFT )
			vec2 texelSize = vec2( 1.0 ) / shadowMapSize;
			float dx = texelSize.x;
			float dy = texelSize.y;
			vec2 uv = shadowCoord.xy;
			vec2 f = fract( uv * shadowMapSize + 0.5 );
			uv -= f * texelSize;
			shadow = (
				texture2DCompare( shadowMap, uv, shadowCoord.z ) +
				texture2DCompare( shadowMap, uv + vec2( dx, 0.0 ), shadowCoord.z ) +
				texture2DCompare( shadowMap, uv + vec2( 0.0, dy ), shadowCoord.z ) +
				texture2DCompare( shadowMap, uv + texelSize, shadowCoord.z ) +
				mix( texture2DCompare( shadowMap, uv + vec2( -dx, 0.0 ), shadowCoord.z ),
					 texture2DCompare( shadowMap, uv + vec2( 2.0 * dx, 0.0 ), shadowCoord.z ),
					 f.x ) +
				mix( texture2DCompare( shadowMap, uv + vec2( -dx, dy ), shadowCoord.z ),
					 texture2DCompare( shadowMap, uv + vec2( 2.0 * dx, dy ), shadowCoord.z ),
					 f.x ) +
				mix( texture2DCompare( shadowMap, uv + vec2( 0.0, -dy ), shadowCoord.z ),
					 texture2DCompare( shadowMap, uv + vec2( 0.0, 2.0 * dy ), shadowCoord.z ),
					 f.y ) +
				mix( texture2DCompare( shadowMap, uv + vec2( dx, -dy ), shadowCoord.z ),
					 texture2DCompare( shadowMap, uv + vec2( dx, 2.0 * dy ), shadowCoord.z ),
					 f.y ) +
				mix( mix( texture2DCompare( shadowMap, uv + vec2( -dx, -dy ), shadowCoord.z ),
						  texture2DCompare( shadowMap, uv + vec2( 2.0 * dx, -dy ), shadowCoord.z ),
						  f.x ),
					 mix( texture2DCompare( shadowMap, uv + vec2( -dx, 2.0 * dy ), shadowCoord.z ),
						  texture2DCompare( shadowMap, uv + vec2( 2.0 * dx, 2.0 * dy ), shadowCoord.z ),
						  f.x ),
					 f.y )
			) * ( 1.0 / 9.0 );
		#elif defined( SHADOWMAP_TYPE_VSM )
			shadow = VSMShadow( shadowMap, shadowCoord.xy, shadowCoord.z );
		#else
			shadow = texture2DCompare( shadowMap, shadowCoord.xy, shadowCoord.z );
		#endif
		}
		return mix( 1.0, shadow, shadowIntensity );
	}
	vec2 cubeToUV( vec3 v, float texelSizeY ) {
		vec3 absV = abs( v );
		float scaleToCube = 1.0 / max( absV.x, max( absV.y, absV.z ) );
		absV *= scaleToCube;
		v *= scaleToCube * ( 1.0 - 2.0 * texelSizeY );
		vec2 planar = v.xy;
		float almostATexel = 1.5 * texelSizeY;
		float almostOne = 1.0 - almostATexel;
		if ( absV.z >= almostOne ) {
			if ( v.z > 0.0 )
				planar.x = 4.0 - v.x;
		} else if ( absV.x >= almostOne ) {
			float signX = sign( v.x );
			planar.x = v.z * signX + 2.0 * signX;
		} else if ( absV.y >= almostOne ) {
			float signY = sign( v.y );
			planar.x = v.x + 2.0 * signY + 2.0;
			planar.y = v.z * signY - 2.0;
		}
		return vec2( 0.125, 0.25 ) * planar + vec2( 0.375, 0.75 );
	}
	float getPointShadow( sampler2D shadowMap, vec2 shadowMapSize, float shadowIntensity, float shadowBias, float shadowRadius, vec4 shadowCoord, float shadowCameraNear, float shadowCameraFar ) {
		float shadow = 1.0;
		vec3 lightToPosition = shadowCoord.xyz;
		
		float lightToPositionLength = length( lightToPosition );
		if ( lightToPositionLength - shadowCameraFar <= 0.0 && lightToPositionLength - shadowCameraNear >= 0.0 ) {
			float dp = ( lightToPositionLength - shadowCameraNear ) / ( shadowCameraFar - shadowCameraNear );			dp += shadowBias;
			vec3 bd3D = normalize( lightToPosition );
			vec2 texelSize = vec2( 1.0 ) / ( shadowMapSize * vec2( 4.0, 2.0 ) );
			#if defined( SHADOWMAP_TYPE_PCF ) || defined( SHADOWMAP_TYPE_PCF_SOFT ) || defined( SHADOWMAP_TYPE_VSM )
				vec2 offset = vec2( - 1, 1 ) * shadowRadius * texelSize.y;
				shadow = (
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.xyy, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.yyy, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.xyx, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.yyx, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.xxy, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.yxy, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.xxx, texelSize.y ), dp ) +
					texture2DCompare( shadowMap, cubeToUV( bd3D + offset.yxx, texelSize.y ), dp )
				) * ( 1.0 / 9.0 );
			#else
				shadow = texture2DCompare( shadowMap, cubeToUV( bd3D, texelSize.y ), dp );
			#endif
		}
		return mix( 1.0, shadow, shadowIntensity );
	}
#endif`,Gm=`#if NUM_SPOT_LIGHT_COORDS > 0
	uniform mat4 spotLightMatrix[ NUM_SPOT_LIGHT_COORDS ];
	varying vec4 vSpotLightCoord[ NUM_SPOT_LIGHT_COORDS ];
#endif
#ifdef USE_SHADOWMAP
	#if NUM_DIR_LIGHT_SHADOWS > 0
		uniform mat4 directionalShadowMatrix[ NUM_DIR_LIGHT_SHADOWS ];
		varying vec4 vDirectionalShadowCoord[ NUM_DIR_LIGHT_SHADOWS ];
		struct DirectionalLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
		};
		uniform DirectionalLightShadow directionalLightShadows[ NUM_DIR_LIGHT_SHADOWS ];
	#endif
	#if NUM_SPOT_LIGHT_SHADOWS > 0
		struct SpotLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
		};
		uniform SpotLightShadow spotLightShadows[ NUM_SPOT_LIGHT_SHADOWS ];
	#endif
	#if NUM_POINT_LIGHT_SHADOWS > 0
		uniform mat4 pointShadowMatrix[ NUM_POINT_LIGHT_SHADOWS ];
		varying vec4 vPointShadowCoord[ NUM_POINT_LIGHT_SHADOWS ];
		struct PointLightShadow {
			float shadowIntensity;
			float shadowBias;
			float shadowNormalBias;
			float shadowRadius;
			vec2 shadowMapSize;
			float shadowCameraNear;
			float shadowCameraFar;
		};
		uniform PointLightShadow pointLightShadows[ NUM_POINT_LIGHT_SHADOWS ];
	#endif
#endif`,Wm=`#if ( defined( USE_SHADOWMAP ) && ( NUM_DIR_LIGHT_SHADOWS > 0 || NUM_POINT_LIGHT_SHADOWS > 0 ) ) || ( NUM_SPOT_LIGHT_COORDS > 0 )
	vec3 shadowWorldNormal = inverseTransformDirection( transformedNormal, viewMatrix );
	vec4 shadowWorldPosition;
#endif
#if defined( USE_SHADOWMAP )
	#if NUM_DIR_LIGHT_SHADOWS > 0
		#pragma unroll_loop_start
		for ( int i = 0; i < NUM_DIR_LIGHT_SHADOWS; i ++ ) {
			shadowWorldPosition = worldPosition + vec4( shadowWorldNormal * directionalLightShadows[ i ].shadowNormalBias, 0 );
			vDirectionalShadowCoord[ i ] = directionalShadowMatrix[ i ] * shadowWorldPosition;
		}
		#pragma unroll_loop_end
	#endif
	#if NUM_POINT_LIGHT_SHADOWS > 0
		#pragma unroll_loop_start
		for ( int i = 0; i < NUM_POINT_LIGHT_SHADOWS; i ++ ) {
			shadowWorldPosition = worldPosition + vec4( shadowWorldNormal * pointLightShadows[ i ].shadowNormalBias, 0 );
			vPointShadowCoord[ i ] = pointShadowMatrix[ i ] * shadowWorldPosition;
		}
		#pragma unroll_loop_end
	#endif
#endif
#if NUM_SPOT_LIGHT_COORDS > 0
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_SPOT_LIGHT_COORDS; i ++ ) {
		shadowWorldPosition = worldPosition;
		#if ( defined( USE_SHADOWMAP ) && UNROLLED_LOOP_INDEX < NUM_SPOT_LIGHT_SHADOWS )
			shadowWorldPosition.xyz += shadowWorldNormal * spotLightShadows[ i ].shadowNormalBias;
		#endif
		vSpotLightCoord[ i ] = spotLightMatrix[ i ] * shadowWorldPosition;
	}
	#pragma unroll_loop_end
#endif`,Xm=`float getShadowMask() {
	float shadow = 1.0;
	#ifdef USE_SHADOWMAP
	#if NUM_DIR_LIGHT_SHADOWS > 0
	DirectionalLightShadow directionalLight;
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_DIR_LIGHT_SHADOWS; i ++ ) {
		directionalLight = directionalLightShadows[ i ];
		shadow *= receiveShadow ? getShadow( directionalShadowMap[ i ], directionalLight.shadowMapSize, directionalLight.shadowIntensity, directionalLight.shadowBias, directionalLight.shadowRadius, vDirectionalShadowCoord[ i ] ) : 1.0;
	}
	#pragma unroll_loop_end
	#endif
	#if NUM_SPOT_LIGHT_SHADOWS > 0
	SpotLightShadow spotLight;
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_SPOT_LIGHT_SHADOWS; i ++ ) {
		spotLight = spotLightShadows[ i ];
		shadow *= receiveShadow ? getShadow( spotShadowMap[ i ], spotLight.shadowMapSize, spotLight.shadowIntensity, spotLight.shadowBias, spotLight.shadowRadius, vSpotLightCoord[ i ] ) : 1.0;
	}
	#pragma unroll_loop_end
	#endif
	#if NUM_POINT_LIGHT_SHADOWS > 0
	PointLightShadow pointLight;
	#pragma unroll_loop_start
	for ( int i = 0; i < NUM_POINT_LIGHT_SHADOWS; i ++ ) {
		pointLight = pointLightShadows[ i ];
		shadow *= receiveShadow ? getPointShadow( pointShadowMap[ i ], pointLight.shadowMapSize, pointLight.shadowIntensity, pointLight.shadowBias, pointLight.shadowRadius, vPointShadowCoord[ i ], pointLight.shadowCameraNear, pointLight.shadowCameraFar ) : 1.0;
	}
	#pragma unroll_loop_end
	#endif
	#endif
	return shadow;
}`,qm=`#ifdef USE_SKINNING
	mat4 boneMatX = getBoneMatrix( skinIndex.x );
	mat4 boneMatY = getBoneMatrix( skinIndex.y );
	mat4 boneMatZ = getBoneMatrix( skinIndex.z );
	mat4 boneMatW = getBoneMatrix( skinIndex.w );
#endif`,Ym=`#ifdef USE_SKINNING
	uniform mat4 bindMatrix;
	uniform mat4 bindMatrixInverse;
	uniform highp sampler2D boneTexture;
	mat4 getBoneMatrix( const in float i ) {
		int size = textureSize( boneTexture, 0 ).x;
		int j = int( i ) * 4;
		int x = j % size;
		int y = j / size;
		vec4 v1 = texelFetch( boneTexture, ivec2( x, y ), 0 );
		vec4 v2 = texelFetch( boneTexture, ivec2( x + 1, y ), 0 );
		vec4 v3 = texelFetch( boneTexture, ivec2( x + 2, y ), 0 );
		vec4 v4 = texelFetch( boneTexture, ivec2( x + 3, y ), 0 );
		return mat4( v1, v2, v3, v4 );
	}
#endif`,Zm=`#ifdef USE_SKINNING
	vec4 skinVertex = bindMatrix * vec4( transformed, 1.0 );
	vec4 skinned = vec4( 0.0 );
	skinned += boneMatX * skinVertex * skinWeight.x;
	skinned += boneMatY * skinVertex * skinWeight.y;
	skinned += boneMatZ * skinVertex * skinWeight.z;
	skinned += boneMatW * skinVertex * skinWeight.w;
	transformed = ( bindMatrixInverse * skinned ).xyz;
#endif`,jm=`#ifdef USE_SKINNING
	mat4 skinMatrix = mat4( 0.0 );
	skinMatrix += skinWeight.x * boneMatX;
	skinMatrix += skinWeight.y * boneMatY;
	skinMatrix += skinWeight.z * boneMatZ;
	skinMatrix += skinWeight.w * boneMatW;
	skinMatrix = bindMatrixInverse * skinMatrix * bindMatrix;
	objectNormal = vec4( skinMatrix * vec4( objectNormal, 0.0 ) ).xyz;
	#ifdef USE_TANGENT
		objectTangent = vec4( skinMatrix * vec4( objectTangent, 0.0 ) ).xyz;
	#endif
#endif`,Km=`float specularStrength;
#ifdef USE_SPECULARMAP
	vec4 texelSpecular = texture2D( specularMap, vSpecularMapUv );
	specularStrength = texelSpecular.r;
#else
	specularStrength = 1.0;
#endif`,Jm=`#ifdef USE_SPECULARMAP
	uniform sampler2D specularMap;
#endif`,Qm=`#if defined( TONE_MAPPING )
	gl_FragColor.rgb = toneMapping( gl_FragColor.rgb );
#endif`,$m=`#ifndef saturate
#define saturate( a ) clamp( a, 0.0, 1.0 )
#endif
uniform float toneMappingExposure;
vec3 LinearToneMapping( vec3 color ) {
	return saturate( toneMappingExposure * color );
}
vec3 ReinhardToneMapping( vec3 color ) {
	color *= toneMappingExposure;
	return saturate( color / ( vec3( 1.0 ) + color ) );
}
vec3 CineonToneMapping( vec3 color ) {
	color *= toneMappingExposure;
	color = max( vec3( 0.0 ), color - 0.004 );
	return pow( ( color * ( 6.2 * color + 0.5 ) ) / ( color * ( 6.2 * color + 1.7 ) + 0.06 ), vec3( 2.2 ) );
}
vec3 RRTAndODTFit( vec3 v ) {
	vec3 a = v * ( v + 0.0245786 ) - 0.000090537;
	vec3 b = v * ( 0.983729 * v + 0.4329510 ) + 0.238081;
	return a / b;
}
vec3 ACESFilmicToneMapping( vec3 color ) {
	const mat3 ACESInputMat = mat3(
		vec3( 0.59719, 0.07600, 0.02840 ),		vec3( 0.35458, 0.90834, 0.13383 ),
		vec3( 0.04823, 0.01566, 0.83777 )
	);
	const mat3 ACESOutputMat = mat3(
		vec3(  1.60475, -0.10208, -0.00327 ),		vec3( -0.53108,  1.10813, -0.07276 ),
		vec3( -0.07367, -0.00605,  1.07602 )
	);
	color *= toneMappingExposure / 0.6;
	color = ACESInputMat * color;
	color = RRTAndODTFit( color );
	color = ACESOutputMat * color;
	return saturate( color );
}
const mat3 LINEAR_REC2020_TO_LINEAR_SRGB = mat3(
	vec3( 1.6605, - 0.1246, - 0.0182 ),
	vec3( - 0.5876, 1.1329, - 0.1006 ),
	vec3( - 0.0728, - 0.0083, 1.1187 )
);
const mat3 LINEAR_SRGB_TO_LINEAR_REC2020 = mat3(
	vec3( 0.6274, 0.0691, 0.0164 ),
	vec3( 0.3293, 0.9195, 0.0880 ),
	vec3( 0.0433, 0.0113, 0.8956 )
);
vec3 agxDefaultContrastApprox( vec3 x ) {
	vec3 x2 = x * x;
	vec3 x4 = x2 * x2;
	return + 15.5 * x4 * x2
		- 40.14 * x4 * x
		+ 31.96 * x4
		- 6.868 * x2 * x
		+ 0.4298 * x2
		+ 0.1191 * x
		- 0.00232;
}
vec3 AgXToneMapping( vec3 color ) {
	const mat3 AgXInsetMatrix = mat3(
		vec3( 0.856627153315983, 0.137318972929847, 0.11189821299995 ),
		vec3( 0.0951212405381588, 0.761241990602591, 0.0767994186031903 ),
		vec3( 0.0482516061458583, 0.101439036467562, 0.811302368396859 )
	);
	const mat3 AgXOutsetMatrix = mat3(
		vec3( 1.1271005818144368, - 0.1413297634984383, - 0.14132976349843826 ),
		vec3( - 0.11060664309660323, 1.157823702216272, - 0.11060664309660294 ),
		vec3( - 0.016493938717834573, - 0.016493938717834257, 1.2519364065950405 )
	);
	const float AgxMinEv = - 12.47393;	const float AgxMaxEv = 4.026069;
	color *= toneMappingExposure;
	color = LINEAR_SRGB_TO_LINEAR_REC2020 * color;
	color = AgXInsetMatrix * color;
	color = max( color, 1e-10 );	color = log2( color );
	color = ( color - AgxMinEv ) / ( AgxMaxEv - AgxMinEv );
	color = clamp( color, 0.0, 1.0 );
	color = agxDefaultContrastApprox( color );
	color = AgXOutsetMatrix * color;
	color = pow( max( vec3( 0.0 ), color ), vec3( 2.2 ) );
	color = LINEAR_REC2020_TO_LINEAR_SRGB * color;
	color = clamp( color, 0.0, 1.0 );
	return color;
}
vec3 NeutralToneMapping( vec3 color ) {
	const float StartCompression = 0.8 - 0.04;
	const float Desaturation = 0.15;
	color *= toneMappingExposure;
	float x = min( color.r, min( color.g, color.b ) );
	float offset = x < 0.08 ? x - 6.25 * x * x : 0.04;
	color -= offset;
	float peak = max( color.r, max( color.g, color.b ) );
	if ( peak < StartCompression ) return color;
	float d = 1. - StartCompression;
	float newPeak = 1. - d * d / ( peak + d - StartCompression );
	color *= newPeak / peak;
	float g = 1. - 1. / ( Desaturation * ( peak - newPeak ) + 1. );
	return mix( color, vec3( newPeak ), g );
}
vec3 CustomToneMapping( vec3 color ) { return color; }`,tg=`#ifdef USE_TRANSMISSION
	material.transmission = transmission;
	material.transmissionAlpha = 1.0;
	material.thickness = thickness;
	material.attenuationDistance = attenuationDistance;
	material.attenuationColor = attenuationColor;
	#ifdef USE_TRANSMISSIONMAP
		material.transmission *= texture2D( transmissionMap, vTransmissionMapUv ).r;
	#endif
	#ifdef USE_THICKNESSMAP
		material.thickness *= texture2D( thicknessMap, vThicknessMapUv ).g;
	#endif
	vec3 pos = vWorldPosition;
	vec3 v = normalize( cameraPosition - pos );
	vec3 n = inverseTransformDirection( normal, viewMatrix );
	vec4 transmitted = getIBLVolumeRefraction(
		n, v, material.roughness, material.diffuseColor, material.specularColor, material.specularF90,
		pos, modelMatrix, viewMatrix, projectionMatrix, material.dispersion, material.ior, material.thickness,
		material.attenuationColor, material.attenuationDistance );
	material.transmissionAlpha = mix( material.transmissionAlpha, transmitted.a, material.transmission );
	totalDiffuse = mix( totalDiffuse, transmitted.rgb, material.transmission );
#endif`,eg=`#ifdef USE_TRANSMISSION
	uniform float transmission;
	uniform float thickness;
	uniform float attenuationDistance;
	uniform vec3 attenuationColor;
	#ifdef USE_TRANSMISSIONMAP
		uniform sampler2D transmissionMap;
	#endif
	#ifdef USE_THICKNESSMAP
		uniform sampler2D thicknessMap;
	#endif
	uniform vec2 transmissionSamplerSize;
	uniform sampler2D transmissionSamplerMap;
	uniform mat4 modelMatrix;
	uniform mat4 projectionMatrix;
	varying vec3 vWorldPosition;
	float w0( float a ) {
		return ( 1.0 / 6.0 ) * ( a * ( a * ( - a + 3.0 ) - 3.0 ) + 1.0 );
	}
	float w1( float a ) {
		return ( 1.0 / 6.0 ) * ( a *  a * ( 3.0 * a - 6.0 ) + 4.0 );
	}
	float w2( float a ){
		return ( 1.0 / 6.0 ) * ( a * ( a * ( - 3.0 * a + 3.0 ) + 3.0 ) + 1.0 );
	}
	float w3( float a ) {
		return ( 1.0 / 6.0 ) * ( a * a * a );
	}
	float g0( float a ) {
		return w0( a ) + w1( a );
	}
	float g1( float a ) {
		return w2( a ) + w3( a );
	}
	float h0( float a ) {
		return - 1.0 + w1( a ) / ( w0( a ) + w1( a ) );
	}
	float h1( float a ) {
		return 1.0 + w3( a ) / ( w2( a ) + w3( a ) );
	}
	vec4 bicubic( sampler2D tex, vec2 uv, vec4 texelSize, float lod ) {
		uv = uv * texelSize.zw + 0.5;
		vec2 iuv = floor( uv );
		vec2 fuv = fract( uv );
		float g0x = g0( fuv.x );
		float g1x = g1( fuv.x );
		float h0x = h0( fuv.x );
		float h1x = h1( fuv.x );
		float h0y = h0( fuv.y );
		float h1y = h1( fuv.y );
		vec2 p0 = ( vec2( iuv.x + h0x, iuv.y + h0y ) - 0.5 ) * texelSize.xy;
		vec2 p1 = ( vec2( iuv.x + h1x, iuv.y + h0y ) - 0.5 ) * texelSize.xy;
		vec2 p2 = ( vec2( iuv.x + h0x, iuv.y + h1y ) - 0.5 ) * texelSize.xy;
		vec2 p3 = ( vec2( iuv.x + h1x, iuv.y + h1y ) - 0.5 ) * texelSize.xy;
		return g0( fuv.y ) * ( g0x * textureLod( tex, p0, lod ) + g1x * textureLod( tex, p1, lod ) ) +
			g1( fuv.y ) * ( g0x * textureLod( tex, p2, lod ) + g1x * textureLod( tex, p3, lod ) );
	}
	vec4 textureBicubic( sampler2D sampler, vec2 uv, float lod ) {
		vec2 fLodSize = vec2( textureSize( sampler, int( lod ) ) );
		vec2 cLodSize = vec2( textureSize( sampler, int( lod + 1.0 ) ) );
		vec2 fLodSizeInv = 1.0 / fLodSize;
		vec2 cLodSizeInv = 1.0 / cLodSize;
		vec4 fSample = bicubic( sampler, uv, vec4( fLodSizeInv, fLodSize ), floor( lod ) );
		vec4 cSample = bicubic( sampler, uv, vec4( cLodSizeInv, cLodSize ), ceil( lod ) );
		return mix( fSample, cSample, fract( lod ) );
	}
	vec3 getVolumeTransmissionRay( const in vec3 n, const in vec3 v, const in float thickness, const in float ior, const in mat4 modelMatrix ) {
		vec3 refractionVector = refract( - v, normalize( n ), 1.0 / ior );
		vec3 modelScale;
		modelScale.x = length( vec3( modelMatrix[ 0 ].xyz ) );
		modelScale.y = length( vec3( modelMatrix[ 1 ].xyz ) );
		modelScale.z = length( vec3( modelMatrix[ 2 ].xyz ) );
		return normalize( refractionVector ) * thickness * modelScale;
	}
	float applyIorToRoughness( const in float roughness, const in float ior ) {
		return roughness * clamp( ior * 2.0 - 2.0, 0.0, 1.0 );
	}
	vec4 getTransmissionSample( const in vec2 fragCoord, const in float roughness, const in float ior ) {
		float lod = log2( transmissionSamplerSize.x ) * applyIorToRoughness( roughness, ior );
		return textureBicubic( transmissionSamplerMap, fragCoord.xy, lod );
	}
	vec3 volumeAttenuation( const in float transmissionDistance, const in vec3 attenuationColor, const in float attenuationDistance ) {
		if ( isinf( attenuationDistance ) ) {
			return vec3( 1.0 );
		} else {
			vec3 attenuationCoefficient = -log( attenuationColor ) / attenuationDistance;
			vec3 transmittance = exp( - attenuationCoefficient * transmissionDistance );			return transmittance;
		}
	}
	vec4 getIBLVolumeRefraction( const in vec3 n, const in vec3 v, const in float roughness, const in vec3 diffuseColor,
		const in vec3 specularColor, const in float specularF90, const in vec3 position, const in mat4 modelMatrix,
		const in mat4 viewMatrix, const in mat4 projMatrix, const in float dispersion, const in float ior, const in float thickness,
		const in vec3 attenuationColor, const in float attenuationDistance ) {
		vec4 transmittedLight;
		vec3 transmittance;
		#ifdef USE_DISPERSION
			float halfSpread = ( ior - 1.0 ) * 0.025 * dispersion;
			vec3 iors = vec3( ior - halfSpread, ior, ior + halfSpread );
			for ( int i = 0; i < 3; i ++ ) {
				vec3 transmissionRay = getVolumeTransmissionRay( n, v, thickness, iors[ i ], modelMatrix );
				vec3 refractedRayExit = position + transmissionRay;
		
				vec4 ndcPos = projMatrix * viewMatrix * vec4( refractedRayExit, 1.0 );
				vec2 refractionCoords = ndcPos.xy / ndcPos.w;
				refractionCoords += 1.0;
				refractionCoords /= 2.0;
		
				vec4 transmissionSample = getTransmissionSample( refractionCoords, roughness, iors[ i ] );
				transmittedLight[ i ] = transmissionSample[ i ];
				transmittedLight.a += transmissionSample.a;
				transmittance[ i ] = diffuseColor[ i ] * volumeAttenuation( length( transmissionRay ), attenuationColor, attenuationDistance )[ i ];
			}
			transmittedLight.a /= 3.0;
		
		#else
		
			vec3 transmissionRay = getVolumeTransmissionRay( n, v, thickness, ior, modelMatrix );
			vec3 refractedRayExit = position + transmissionRay;
			vec4 ndcPos = projMatrix * viewMatrix * vec4( refractedRayExit, 1.0 );
			vec2 refractionCoords = ndcPos.xy / ndcPos.w;
			refractionCoords += 1.0;
			refractionCoords /= 2.0;
			transmittedLight = getTransmissionSample( refractionCoords, roughness, ior );
			transmittance = diffuseColor * volumeAttenuation( length( transmissionRay ), attenuationColor, attenuationDistance );
		
		#endif
		vec3 attenuatedColor = transmittance * transmittedLight.rgb;
		vec3 F = EnvironmentBRDF( n, v, specularColor, specularF90, roughness );
		float transmittanceFactor = ( transmittance.r + transmittance.g + transmittance.b ) / 3.0;
		return vec4( ( 1.0 - F ) * attenuatedColor, 1.0 - ( 1.0 - transmittedLight.a ) * transmittanceFactor );
	}
#endif`,ng=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
	varying vec2 vUv;
#endif
#ifdef USE_MAP
	varying vec2 vMapUv;
#endif
#ifdef USE_ALPHAMAP
	varying vec2 vAlphaMapUv;
#endif
#ifdef USE_LIGHTMAP
	varying vec2 vLightMapUv;
#endif
#ifdef USE_AOMAP
	varying vec2 vAoMapUv;
#endif
#ifdef USE_BUMPMAP
	varying vec2 vBumpMapUv;
#endif
#ifdef USE_NORMALMAP
	varying vec2 vNormalMapUv;
#endif
#ifdef USE_EMISSIVEMAP
	varying vec2 vEmissiveMapUv;
#endif
#ifdef USE_METALNESSMAP
	varying vec2 vMetalnessMapUv;
#endif
#ifdef USE_ROUGHNESSMAP
	varying vec2 vRoughnessMapUv;
#endif
#ifdef USE_ANISOTROPYMAP
	varying vec2 vAnisotropyMapUv;
#endif
#ifdef USE_CLEARCOATMAP
	varying vec2 vClearcoatMapUv;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	varying vec2 vClearcoatNormalMapUv;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	varying vec2 vClearcoatRoughnessMapUv;
#endif
#ifdef USE_IRIDESCENCEMAP
	varying vec2 vIridescenceMapUv;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	varying vec2 vIridescenceThicknessMapUv;
#endif
#ifdef USE_SHEEN_COLORMAP
	varying vec2 vSheenColorMapUv;
#endif
#ifdef USE_SHEEN_ROUGHNESSMAP
	varying vec2 vSheenRoughnessMapUv;
#endif
#ifdef USE_SPECULARMAP
	varying vec2 vSpecularMapUv;
#endif
#ifdef USE_SPECULAR_COLORMAP
	varying vec2 vSpecularColorMapUv;
#endif
#ifdef USE_SPECULAR_INTENSITYMAP
	varying vec2 vSpecularIntensityMapUv;
#endif
#ifdef USE_TRANSMISSIONMAP
	uniform mat3 transmissionMapTransform;
	varying vec2 vTransmissionMapUv;
#endif
#ifdef USE_THICKNESSMAP
	uniform mat3 thicknessMapTransform;
	varying vec2 vThicknessMapUv;
#endif`,ig=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
	varying vec2 vUv;
#endif
#ifdef USE_MAP
	uniform mat3 mapTransform;
	varying vec2 vMapUv;
#endif
#ifdef USE_ALPHAMAP
	uniform mat3 alphaMapTransform;
	varying vec2 vAlphaMapUv;
#endif
#ifdef USE_LIGHTMAP
	uniform mat3 lightMapTransform;
	varying vec2 vLightMapUv;
#endif
#ifdef USE_AOMAP
	uniform mat3 aoMapTransform;
	varying vec2 vAoMapUv;
#endif
#ifdef USE_BUMPMAP
	uniform mat3 bumpMapTransform;
	varying vec2 vBumpMapUv;
#endif
#ifdef USE_NORMALMAP
	uniform mat3 normalMapTransform;
	varying vec2 vNormalMapUv;
#endif
#ifdef USE_DISPLACEMENTMAP
	uniform mat3 displacementMapTransform;
	varying vec2 vDisplacementMapUv;
#endif
#ifdef USE_EMISSIVEMAP
	uniform mat3 emissiveMapTransform;
	varying vec2 vEmissiveMapUv;
#endif
#ifdef USE_METALNESSMAP
	uniform mat3 metalnessMapTransform;
	varying vec2 vMetalnessMapUv;
#endif
#ifdef USE_ROUGHNESSMAP
	uniform mat3 roughnessMapTransform;
	varying vec2 vRoughnessMapUv;
#endif
#ifdef USE_ANISOTROPYMAP
	uniform mat3 anisotropyMapTransform;
	varying vec2 vAnisotropyMapUv;
#endif
#ifdef USE_CLEARCOATMAP
	uniform mat3 clearcoatMapTransform;
	varying vec2 vClearcoatMapUv;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform mat3 clearcoatNormalMapTransform;
	varying vec2 vClearcoatNormalMapUv;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform mat3 clearcoatRoughnessMapTransform;
	varying vec2 vClearcoatRoughnessMapUv;
#endif
#ifdef USE_SHEEN_COLORMAP
	uniform mat3 sheenColorMapTransform;
	varying vec2 vSheenColorMapUv;
#endif
#ifdef USE_SHEEN_ROUGHNESSMAP
	uniform mat3 sheenRoughnessMapTransform;
	varying vec2 vSheenRoughnessMapUv;
#endif
#ifdef USE_IRIDESCENCEMAP
	uniform mat3 iridescenceMapTransform;
	varying vec2 vIridescenceMapUv;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform mat3 iridescenceThicknessMapTransform;
	varying vec2 vIridescenceThicknessMapUv;
#endif
#ifdef USE_SPECULARMAP
	uniform mat3 specularMapTransform;
	varying vec2 vSpecularMapUv;
#endif
#ifdef USE_SPECULAR_COLORMAP
	uniform mat3 specularColorMapTransform;
	varying vec2 vSpecularColorMapUv;
#endif
#ifdef USE_SPECULAR_INTENSITYMAP
	uniform mat3 specularIntensityMapTransform;
	varying vec2 vSpecularIntensityMapUv;
#endif
#ifdef USE_TRANSMISSIONMAP
	uniform mat3 transmissionMapTransform;
	varying vec2 vTransmissionMapUv;
#endif
#ifdef USE_THICKNESSMAP
	uniform mat3 thicknessMapTransform;
	varying vec2 vThicknessMapUv;
#endif`,sg=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
	vUv = vec3( uv, 1 ).xy;
#endif
#ifdef USE_MAP
	vMapUv = ( mapTransform * vec3( MAP_UV, 1 ) ).xy;
#endif
#ifdef USE_ALPHAMAP
	vAlphaMapUv = ( alphaMapTransform * vec3( ALPHAMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_LIGHTMAP
	vLightMapUv = ( lightMapTransform * vec3( LIGHTMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_AOMAP
	vAoMapUv = ( aoMapTransform * vec3( AOMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_BUMPMAP
	vBumpMapUv = ( bumpMapTransform * vec3( BUMPMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_NORMALMAP
	vNormalMapUv = ( normalMapTransform * vec3( NORMALMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_DISPLACEMENTMAP
	vDisplacementMapUv = ( displacementMapTransform * vec3( DISPLACEMENTMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_EMISSIVEMAP
	vEmissiveMapUv = ( emissiveMapTransform * vec3( EMISSIVEMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_METALNESSMAP
	vMetalnessMapUv = ( metalnessMapTransform * vec3( METALNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_ROUGHNESSMAP
	vRoughnessMapUv = ( roughnessMapTransform * vec3( ROUGHNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_ANISOTROPYMAP
	vAnisotropyMapUv = ( anisotropyMapTransform * vec3( ANISOTROPYMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_CLEARCOATMAP
	vClearcoatMapUv = ( clearcoatMapTransform * vec3( CLEARCOATMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	vClearcoatNormalMapUv = ( clearcoatNormalMapTransform * vec3( CLEARCOAT_NORMALMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	vClearcoatRoughnessMapUv = ( clearcoatRoughnessMapTransform * vec3( CLEARCOAT_ROUGHNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_IRIDESCENCEMAP
	vIridescenceMapUv = ( iridescenceMapTransform * vec3( IRIDESCENCEMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	vIridescenceThicknessMapUv = ( iridescenceThicknessMapTransform * vec3( IRIDESCENCE_THICKNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SHEEN_COLORMAP
	vSheenColorMapUv = ( sheenColorMapTransform * vec3( SHEEN_COLORMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SHEEN_ROUGHNESSMAP
	vSheenRoughnessMapUv = ( sheenRoughnessMapTransform * vec3( SHEEN_ROUGHNESSMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SPECULARMAP
	vSpecularMapUv = ( specularMapTransform * vec3( SPECULARMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SPECULAR_COLORMAP
	vSpecularColorMapUv = ( specularColorMapTransform * vec3( SPECULAR_COLORMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_SPECULAR_INTENSITYMAP
	vSpecularIntensityMapUv = ( specularIntensityMapTransform * vec3( SPECULAR_INTENSITYMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_TRANSMISSIONMAP
	vTransmissionMapUv = ( transmissionMapTransform * vec3( TRANSMISSIONMAP_UV, 1 ) ).xy;
#endif
#ifdef USE_THICKNESSMAP
	vThicknessMapUv = ( thicknessMapTransform * vec3( THICKNESSMAP_UV, 1 ) ).xy;
#endif`,rg=`#if defined( USE_ENVMAP ) || defined( DISTANCE ) || defined ( USE_SHADOWMAP ) || defined ( USE_TRANSMISSION ) || NUM_SPOT_LIGHT_COORDS > 0
	vec4 worldPosition = vec4( transformed, 1.0 );
	#ifdef USE_BATCHING
		worldPosition = batchingMatrix * worldPosition;
	#endif
	#ifdef USE_INSTANCING
		worldPosition = instanceMatrix * worldPosition;
	#endif
	worldPosition = modelMatrix * worldPosition;
#endif`,og=`varying vec2 vUv;
uniform mat3 uvTransform;
void main() {
	vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	gl_Position = vec4( position.xy, 1.0, 1.0 );
}`,ag=`uniform sampler2D t2D;
uniform float backgroundIntensity;
varying vec2 vUv;
void main() {
	vec4 texColor = texture2D( t2D, vUv );
	#ifdef DECODE_VIDEO_TEXTURE
		texColor = vec4( mix( pow( texColor.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), texColor.rgb * 0.0773993808, vec3( lessThanEqual( texColor.rgb, vec3( 0.04045 ) ) ) ), texColor.w );
	#endif
	texColor.rgb *= backgroundIntensity;
	gl_FragColor = texColor;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,lg=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,cg=`#ifdef ENVMAP_TYPE_CUBE
	uniform samplerCube envMap;
#elif defined( ENVMAP_TYPE_CUBE_UV )
	uniform sampler2D envMap;
#endif
uniform float flipEnvMap;
uniform float backgroundBlurriness;
uniform float backgroundIntensity;
uniform mat3 backgroundRotation;
varying vec3 vWorldDirection;
#include <cube_uv_reflection_fragment>
void main() {
	#ifdef ENVMAP_TYPE_CUBE
		vec4 texColor = textureCube( envMap, backgroundRotation * vec3( flipEnvMap * vWorldDirection.x, vWorldDirection.yz ) );
	#elif defined( ENVMAP_TYPE_CUBE_UV )
		vec4 texColor = textureCubeUV( envMap, backgroundRotation * vWorldDirection, backgroundBlurriness );
	#else
		vec4 texColor = vec4( 0.0, 0.0, 0.0, 1.0 );
	#endif
	texColor.rgb *= backgroundIntensity;
	gl_FragColor = texColor;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,hg=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,ug=`uniform samplerCube tCube;
uniform float tFlip;
uniform float opacity;
varying vec3 vWorldDirection;
void main() {
	vec4 texColor = textureCube( tCube, vec3( tFlip * vWorldDirection.x, vWorldDirection.yz ) );
	gl_FragColor = texColor;
	gl_FragColor.a *= opacity;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,dg=`#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
varying vec2 vHighPrecisionZW;
void main() {
	#include <uv_vertex>
	#include <batching_vertex>
	#include <skinbase_vertex>
	#include <morphinstance_vertex>
	#ifdef USE_DISPLACEMENTMAP
		#include <beginnormal_vertex>
		#include <morphnormal_vertex>
		#include <skinnormal_vertex>
	#endif
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vHighPrecisionZW = gl_Position.zw;
}`,fg=`#if DEPTH_PACKING == 3200
	uniform float opacity;
#endif
#include <common>
#include <packing>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
varying vec2 vHighPrecisionZW;
void main() {
	vec4 diffuseColor = vec4( 1.0 );
	#include <clipping_planes_fragment>
	#if DEPTH_PACKING == 3200
		diffuseColor.a = opacity;
	#endif
	#include <map_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <logdepthbuf_fragment>
	float fragCoordZ = 0.5 * vHighPrecisionZW[0] / vHighPrecisionZW[1] + 0.5;
	#if DEPTH_PACKING == 3200
		gl_FragColor = vec4( vec3( 1.0 - fragCoordZ ), opacity );
	#elif DEPTH_PACKING == 3201
		gl_FragColor = packDepthToRGBA( fragCoordZ );
	#elif DEPTH_PACKING == 3202
		gl_FragColor = vec4( packDepthToRGB( fragCoordZ ), 1.0 );
	#elif DEPTH_PACKING == 3203
		gl_FragColor = vec4( packDepthToRG( fragCoordZ ), 0.0, 1.0 );
	#endif
}`,pg=`#define DISTANCE
varying vec3 vWorldPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <batching_vertex>
	#include <skinbase_vertex>
	#include <morphinstance_vertex>
	#ifdef USE_DISPLACEMENTMAP
		#include <beginnormal_vertex>
		#include <morphnormal_vertex>
		#include <skinnormal_vertex>
	#endif
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <worldpos_vertex>
	#include <clipping_planes_vertex>
	vWorldPosition = worldPosition.xyz;
}`,mg=`#define DISTANCE
uniform vec3 referencePosition;
uniform float nearDistance;
uniform float farDistance;
varying vec3 vWorldPosition;
#include <common>
#include <packing>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <clipping_planes_pars_fragment>
void main () {
	vec4 diffuseColor = vec4( 1.0 );
	#include <clipping_planes_fragment>
	#include <map_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	float dist = length( vWorldPosition - referencePosition );
	dist = ( dist - nearDistance ) / ( farDistance - nearDistance );
	dist = saturate( dist );
	gl_FragColor = packDepthToRGBA( dist );
}`,gg=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
}`,xg=`uniform sampler2D tEquirect;
varying vec3 vWorldDirection;
#include <common>
void main() {
	vec3 direction = normalize( vWorldDirection );
	vec2 sampleUV = equirectUv( direction );
	gl_FragColor = texture2D( tEquirect, sampleUV );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,vg=`uniform float scale;
attribute float lineDistance;
varying float vLineDistance;
#include <common>
#include <uv_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <morphtarget_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	vLineDistance = scale * lineDistance;
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <fog_vertex>
}`,_g=`uniform vec3 diffuse;
uniform float opacity;
uniform float dashSize;
uniform float totalSize;
varying float vLineDistance;
#include <common>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <fog_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	if ( mod( vLineDistance, totalSize ) > dashSize ) {
		discard;
	}
	vec3 outgoingLight = vec3( 0.0 );
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	outgoingLight = diffuseColor.rgb;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
}`,yg=`#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <envmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#if defined ( USE_ENVMAP ) || defined ( USE_SKINNING )
		#include <beginnormal_vertex>
		#include <morphnormal_vertex>
		#include <skinbase_vertex>
		#include <skinnormal_vertex>
		#include <defaultnormal_vertex>
	#endif
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <worldpos_vertex>
	#include <envmap_vertex>
	#include <fog_vertex>
}`,Mg=`uniform vec3 diffuse;
uniform float opacity;
#ifndef FLAT_SHADED
	varying vec3 vNormal;
#endif
#include <common>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_pars_fragment>
#include <fog_pars_fragment>
#include <specularmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <specularmap_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	#ifdef USE_LIGHTMAP
		vec4 lightMapTexel = texture2D( lightMap, vLightMapUv );
		reflectedLight.indirectDiffuse += lightMapTexel.rgb * lightMapIntensity * RECIPROCAL_PI;
	#else
		reflectedLight.indirectDiffuse += vec3( 1.0 );
	#endif
	#include <aomap_fragment>
	reflectedLight.indirectDiffuse *= diffuseColor.rgb;
	vec3 outgoingLight = reflectedLight.indirectDiffuse;
	#include <envmap_fragment>
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,Sg=`#define LAMBERT
varying vec3 vViewPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <envmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <shadowmap_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vViewPosition = - mvPosition.xyz;
	#include <worldpos_vertex>
	#include <envmap_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
}`,bg=`#define LAMBERT
uniform vec3 diffuse;
uniform vec3 emissive;
uniform float opacity;
#include <common>
#include <packing>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <emissivemap_pars_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_pars_fragment>
#include <fog_pars_fragment>
#include <bsdfs>
#include <lights_pars_begin>
#include <normal_pars_fragment>
#include <lights_lambert_pars_fragment>
#include <shadowmap_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <specularmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	vec3 totalEmissiveRadiance = emissive;
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <specularmap_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	#include <emissivemap_fragment>
	#include <lights_lambert_fragment>
	#include <lights_fragment_begin>
	#include <lights_fragment_maps>
	#include <lights_fragment_end>
	#include <aomap_fragment>
	vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse + totalEmissiveRadiance;
	#include <envmap_fragment>
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,wg=`#define MATCAP
varying vec3 vViewPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <color_pars_vertex>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <fog_vertex>
	vViewPosition = - mvPosition.xyz;
}`,Ag=`#define MATCAP
uniform vec3 diffuse;
uniform float opacity;
uniform sampler2D matcap;
varying vec3 vViewPosition;
#include <common>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <fog_pars_fragment>
#include <normal_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	vec3 viewDir = normalize( vViewPosition );
	vec3 x = normalize( vec3( viewDir.z, 0.0, - viewDir.x ) );
	vec3 y = cross( viewDir, x );
	vec2 uv = vec2( dot( x, normal ), dot( y, normal ) ) * 0.495 + 0.5;
	#ifdef USE_MATCAP
		vec4 matcapColor = texture2D( matcap, uv );
	#else
		vec4 matcapColor = vec4( vec3( mix( 0.2, 0.8, uv.y ) ), 1.0 );
	#endif
	vec3 outgoingLight = diffuseColor.rgb * matcapColor.rgb;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,Eg=`#define NORMAL
#if defined( FLAT_SHADED ) || defined( USE_BUMPMAP ) || defined( USE_NORMALMAP_TANGENTSPACE )
	varying vec3 vViewPosition;
#endif
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphinstance_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
#if defined( FLAT_SHADED ) || defined( USE_BUMPMAP ) || defined( USE_NORMALMAP_TANGENTSPACE )
	vViewPosition = - mvPosition.xyz;
#endif
}`,Tg=`#define NORMAL
uniform float opacity;
#if defined( FLAT_SHADED ) || defined( USE_BUMPMAP ) || defined( USE_NORMALMAP_TANGENTSPACE )
	varying vec3 vViewPosition;
#endif
#include <packing>
#include <uv_pars_fragment>
#include <normal_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( 0.0, 0.0, 0.0, opacity );
	#include <clipping_planes_fragment>
	#include <logdepthbuf_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	gl_FragColor = vec4( packNormalToRGB( normal ), diffuseColor.a );
	#ifdef OPAQUE
		gl_FragColor.a = 1.0;
	#endif
}`,Cg=`#define PHONG
varying vec3 vViewPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <envmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <shadowmap_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphinstance_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vViewPosition = - mvPosition.xyz;
	#include <worldpos_vertex>
	#include <envmap_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
}`,Rg=`#define PHONG
uniform vec3 diffuse;
uniform vec3 emissive;
uniform vec3 specular;
uniform float shininess;
uniform float opacity;
#include <common>
#include <packing>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <emissivemap_pars_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_pars_fragment>
#include <fog_pars_fragment>
#include <bsdfs>
#include <lights_pars_begin>
#include <normal_pars_fragment>
#include <lights_phong_pars_fragment>
#include <shadowmap_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <specularmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	vec3 totalEmissiveRadiance = emissive;
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <specularmap_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	#include <emissivemap_fragment>
	#include <lights_phong_fragment>
	#include <lights_fragment_begin>
	#include <lights_fragment_maps>
	#include <lights_fragment_end>
	#include <aomap_fragment>
	vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse + reflectedLight.directSpecular + reflectedLight.indirectSpecular + totalEmissiveRadiance;
	#include <envmap_fragment>
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,Pg=`#define STANDARD
varying vec3 vViewPosition;
#ifdef USE_TRANSMISSION
	varying vec3 vWorldPosition;
#endif
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <shadowmap_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vViewPosition = - mvPosition.xyz;
	#include <worldpos_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
#ifdef USE_TRANSMISSION
	vWorldPosition = worldPosition.xyz;
#endif
}`,Ig=`#define STANDARD
#ifdef PHYSICAL
	#define IOR
	#define USE_SPECULAR
#endif
uniform vec3 diffuse;
uniform vec3 emissive;
uniform float roughness;
uniform float metalness;
uniform float opacity;
#ifdef IOR
	uniform float ior;
#endif
#ifdef USE_SPECULAR
	uniform float specularIntensity;
	uniform vec3 specularColor;
	#ifdef USE_SPECULAR_COLORMAP
		uniform sampler2D specularColorMap;
	#endif
	#ifdef USE_SPECULAR_INTENSITYMAP
		uniform sampler2D specularIntensityMap;
	#endif
#endif
#ifdef USE_CLEARCOAT
	uniform float clearcoat;
	uniform float clearcoatRoughness;
#endif
#ifdef USE_DISPERSION
	uniform float dispersion;
#endif
#ifdef USE_IRIDESCENCE
	uniform float iridescence;
	uniform float iridescenceIOR;
	uniform float iridescenceThicknessMinimum;
	uniform float iridescenceThicknessMaximum;
#endif
#ifdef USE_SHEEN
	uniform vec3 sheenColor;
	uniform float sheenRoughness;
	#ifdef USE_SHEEN_COLORMAP
		uniform sampler2D sheenColorMap;
	#endif
	#ifdef USE_SHEEN_ROUGHNESSMAP
		uniform sampler2D sheenRoughnessMap;
	#endif
#endif
#ifdef USE_ANISOTROPY
	uniform vec2 anisotropyVector;
	#ifdef USE_ANISOTROPYMAP
		uniform sampler2D anisotropyMap;
	#endif
#endif
varying vec3 vViewPosition;
#include <common>
#include <packing>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <emissivemap_pars_fragment>
#include <iridescence_fragment>
#include <cube_uv_reflection_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <lights_pars_begin>
#include <normal_pars_fragment>
#include <lights_physical_pars_fragment>
#include <transmission_pars_fragment>
#include <shadowmap_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <clearcoat_pars_fragment>
#include <iridescence_pars_fragment>
#include <roughnessmap_pars_fragment>
#include <metalnessmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	vec3 totalEmissiveRadiance = emissive;
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <roughnessmap_fragment>
	#include <metalnessmap_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	#include <clearcoat_normal_fragment_begin>
	#include <clearcoat_normal_fragment_maps>
	#include <emissivemap_fragment>
	#include <lights_physical_fragment>
	#include <lights_fragment_begin>
	#include <lights_fragment_maps>
	#include <lights_fragment_end>
	#include <aomap_fragment>
	vec3 totalDiffuse = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse;
	vec3 totalSpecular = reflectedLight.directSpecular + reflectedLight.indirectSpecular;
	#include <transmission_fragment>
	vec3 outgoingLight = totalDiffuse + totalSpecular + totalEmissiveRadiance;
	#ifdef USE_SHEEN
		float sheenEnergyComp = 1.0 - 0.157 * max3( material.sheenColor );
		outgoingLight = outgoingLight * sheenEnergyComp + sheenSpecularDirect + sheenSpecularIndirect;
	#endif
	#ifdef USE_CLEARCOAT
		float dotNVcc = saturate( dot( geometryClearcoatNormal, geometryViewDir ) );
		vec3 Fcc = F_Schlick( material.clearcoatF0, material.clearcoatF90, dotNVcc );
		outgoingLight = outgoingLight * ( 1.0 - material.clearcoat * Fcc ) + ( clearcoatSpecularDirect + clearcoatSpecularIndirect ) * material.clearcoat;
	#endif
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,Lg=`#define TOON
varying vec3 vViewPosition;
#include <common>
#include <batching_pars_vertex>
#include <uv_pars_vertex>
#include <displacementmap_pars_vertex>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <normal_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <shadowmap_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <normal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <displacementmap_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	vViewPosition = - mvPosition.xyz;
	#include <worldpos_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
}`,Dg=`#define TOON
uniform vec3 diffuse;
uniform vec3 emissive;
uniform float opacity;
#include <common>
#include <packing>
#include <dithering_pars_fragment>
#include <color_pars_fragment>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <aomap_pars_fragment>
#include <lightmap_pars_fragment>
#include <emissivemap_pars_fragment>
#include <gradientmap_pars_fragment>
#include <fog_pars_fragment>
#include <bsdfs>
#include <lights_pars_begin>
#include <normal_pars_fragment>
#include <lights_toon_pars_fragment>
#include <shadowmap_pars_fragment>
#include <bumpmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	ReflectedLight reflectedLight = ReflectedLight( vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ), vec3( 0.0 ) );
	vec3 totalEmissiveRadiance = emissive;
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <color_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	#include <normal_fragment_begin>
	#include <normal_fragment_maps>
	#include <emissivemap_fragment>
	#include <lights_toon_fragment>
	#include <lights_fragment_begin>
	#include <lights_fragment_maps>
	#include <lights_fragment_end>
	#include <aomap_fragment>
	vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse + totalEmissiveRadiance;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
	#include <dithering_fragment>
}`,Ug=`uniform float size;
uniform float scale;
#include <common>
#include <color_pars_vertex>
#include <fog_pars_vertex>
#include <morphtarget_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
#ifdef USE_POINTS_UV
	varying vec2 vUv;
	uniform mat3 uvTransform;
#endif
void main() {
	#ifdef USE_POINTS_UV
		vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	#endif
	#include <color_vertex>
	#include <morphinstance_vertex>
	#include <morphcolor_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <project_vertex>
	gl_PointSize = size;
	#ifdef USE_SIZEATTENUATION
		bool isPerspective = isPerspectiveMatrix( projectionMatrix );
		if ( isPerspective ) gl_PointSize *= ( scale / - mvPosition.z );
	#endif
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <worldpos_vertex>
	#include <fog_vertex>
}`,Ng=`uniform vec3 diffuse;
uniform float opacity;
#include <common>
#include <color_pars_fragment>
#include <map_particle_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <fog_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	vec3 outgoingLight = vec3( 0.0 );
	#include <logdepthbuf_fragment>
	#include <map_particle_fragment>
	#include <color_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	outgoingLight = diffuseColor.rgb;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
	#include <premultiplied_alpha_fragment>
}`,Og=`#include <common>
#include <batching_pars_vertex>
#include <fog_pars_vertex>
#include <morphtarget_pars_vertex>
#include <skinning_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <shadowmap_pars_vertex>
void main() {
	#include <batching_vertex>
	#include <beginnormal_vertex>
	#include <morphinstance_vertex>
	#include <morphnormal_vertex>
	#include <skinbase_vertex>
	#include <skinnormal_vertex>
	#include <defaultnormal_vertex>
	#include <begin_vertex>
	#include <morphtarget_vertex>
	#include <skinning_vertex>
	#include <project_vertex>
	#include <logdepthbuf_vertex>
	#include <worldpos_vertex>
	#include <shadowmap_vertex>
	#include <fog_vertex>
}`,Fg=`uniform vec3 color;
uniform float opacity;
#include <common>
#include <packing>
#include <fog_pars_fragment>
#include <bsdfs>
#include <lights_pars_begin>
#include <logdepthbuf_pars_fragment>
#include <shadowmap_pars_fragment>
#include <shadowmask_pars_fragment>
void main() {
	#include <logdepthbuf_fragment>
	gl_FragColor = vec4( color, opacity * ( 1.0 - getShadowMask() ) );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
}`,Bg=`uniform float rotation;
uniform vec2 center;
#include <common>
#include <uv_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <clipping_planes_pars_vertex>
void main() {
	#include <uv_vertex>
	vec4 mvPosition = modelViewMatrix[ 3 ];
	vec2 scale = vec2( length( modelMatrix[ 0 ].xyz ), length( modelMatrix[ 1 ].xyz ) );
	#ifndef USE_SIZEATTENUATION
		bool isPerspective = isPerspectiveMatrix( projectionMatrix );
		if ( isPerspective ) scale *= - mvPosition.z;
	#endif
	vec2 alignedPosition = ( position.xy - ( center - vec2( 0.5 ) ) ) * scale;
	vec2 rotatedPosition;
	rotatedPosition.x = cos( rotation ) * alignedPosition.x - sin( rotation ) * alignedPosition.y;
	rotatedPosition.y = sin( rotation ) * alignedPosition.x + cos( rotation ) * alignedPosition.y;
	mvPosition.xy += rotatedPosition;
	gl_Position = projectionMatrix * mvPosition;
	#include <logdepthbuf_vertex>
	#include <clipping_planes_vertex>
	#include <fog_vertex>
}`,zg=`uniform vec3 diffuse;
uniform float opacity;
#include <common>
#include <uv_pars_fragment>
#include <map_pars_fragment>
#include <alphamap_pars_fragment>
#include <alphatest_pars_fragment>
#include <alphahash_pars_fragment>
#include <fog_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <clipping_planes_pars_fragment>
void main() {
	vec4 diffuseColor = vec4( diffuse, opacity );
	#include <clipping_planes_fragment>
	vec3 outgoingLight = vec3( 0.0 );
	#include <logdepthbuf_fragment>
	#include <map_fragment>
	#include <alphamap_fragment>
	#include <alphatest_fragment>
	#include <alphahash_fragment>
	outgoingLight = diffuseColor.rgb;
	#include <opaque_fragment>
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
	#include <fog_fragment>
}`,Bt={alphahash_fragment:ap,alphahash_pars_fragment:lp,alphamap_fragment:cp,alphamap_pars_fragment:hp,alphatest_fragment:up,alphatest_pars_fragment:dp,aomap_fragment:fp,aomap_pars_fragment:pp,batching_pars_vertex:mp,batching_vertex:gp,begin_vertex:xp,beginnormal_vertex:vp,bsdfs:_p,iridescence_fragment:yp,bumpmap_pars_fragment:Mp,clipping_planes_fragment:Sp,clipping_planes_pars_fragment:bp,clipping_planes_pars_vertex:wp,clipping_planes_vertex:Ap,color_fragment:Ep,color_pars_fragment:Tp,color_pars_vertex:Cp,color_vertex:Rp,common:Pp,cube_uv_reflection_fragment:Ip,defaultnormal_vertex:Lp,displacementmap_pars_vertex:Dp,displacementmap_vertex:Up,emissivemap_fragment:Np,emissivemap_pars_fragment:Op,colorspace_fragment:Fp,colorspace_pars_fragment:Bp,envmap_fragment:zp,envmap_common_pars_fragment:kp,envmap_pars_fragment:Hp,envmap_pars_vertex:Vp,envmap_physical_pars_fragment:$p,envmap_vertex:Gp,fog_vertex:Wp,fog_pars_vertex:Xp,fog_fragment:qp,fog_pars_fragment:Yp,gradientmap_pars_fragment:Zp,lightmap_pars_fragment:jp,lights_lambert_fragment:Kp,lights_lambert_pars_fragment:Jp,lights_pars_begin:Qp,lights_toon_fragment:tm,lights_toon_pars_fragment:em,lights_phong_fragment:nm,lights_phong_pars_fragment:im,lights_physical_fragment:sm,lights_physical_pars_fragment:rm,lights_fragment_begin:om,lights_fragment_maps:am,lights_fragment_end:lm,logdepthbuf_fragment:cm,logdepthbuf_pars_fragment:hm,logdepthbuf_pars_vertex:um,logdepthbuf_vertex:dm,map_fragment:fm,map_pars_fragment:pm,map_particle_fragment:mm,map_particle_pars_fragment:gm,metalnessmap_fragment:xm,metalnessmap_pars_fragment:vm,morphinstance_vertex:_m,morphcolor_vertex:ym,morphnormal_vertex:Mm,morphtarget_pars_vertex:Sm,morphtarget_vertex:bm,normal_fragment_begin:wm,normal_fragment_maps:Am,normal_pars_fragment:Em,normal_pars_vertex:Tm,normal_vertex:Cm,normalmap_pars_fragment:Rm,clearcoat_normal_fragment_begin:Pm,clearcoat_normal_fragment_maps:Im,clearcoat_pars_fragment:Lm,iridescence_pars_fragment:Dm,opaque_fragment:Um,packing:Nm,premultiplied_alpha_fragment:Om,project_vertex:Fm,dithering_fragment:Bm,dithering_pars_fragment:zm,roughnessmap_fragment:km,roughnessmap_pars_fragment:Hm,shadowmap_pars_fragment:Vm,shadowmap_pars_vertex:Gm,shadowmap_vertex:Wm,shadowmask_pars_fragment:Xm,skinbase_vertex:qm,skinning_pars_vertex:Ym,skinning_vertex:Zm,skinnormal_vertex:jm,specularmap_fragment:Km,specularmap_pars_fragment:Jm,tonemapping_fragment:Qm,tonemapping_pars_fragment:$m,transmission_fragment:tg,transmission_pars_fragment:eg,uv_pars_fragment:ng,uv_pars_vertex:ig,uv_vertex:sg,worldpos_vertex:rg,background_vert:og,background_frag:ag,backgroundCube_vert:lg,backgroundCube_frag:cg,cube_vert:hg,cube_frag:ug,depth_vert:dg,depth_frag:fg,distanceRGBA_vert:pg,distanceRGBA_frag:mg,equirect_vert:gg,equirect_frag:xg,linedashed_vert:vg,linedashed_frag:_g,meshbasic_vert:yg,meshbasic_frag:Mg,meshlambert_vert:Sg,meshlambert_frag:bg,meshmatcap_vert:wg,meshmatcap_frag:Ag,meshnormal_vert:Eg,meshnormal_frag:Tg,meshphong_vert:Cg,meshphong_frag:Rg,meshphysical_vert:Pg,meshphysical_frag:Ig,meshtoon_vert:Lg,meshtoon_frag:Dg,points_vert:Ug,points_frag:Ng,shadow_vert:Og,shadow_frag:Fg,sprite_vert:Bg,sprite_frag:zg},dt={common:{diffuse:{value:new Ot(16777215)},opacity:{value:1},map:{value:null},mapTransform:{value:new zt},alphaMap:{value:null},alphaMapTransform:{value:new zt},alphaTest:{value:0}},specularmap:{specularMap:{value:null},specularMapTransform:{value:new zt}},envmap:{envMap:{value:null},envMapRotation:{value:new zt},flipEnvMap:{value:-1},reflectivity:{value:1},ior:{value:1.5},refractionRatio:{value:.98}},aomap:{aoMap:{value:null},aoMapIntensity:{value:1},aoMapTransform:{value:new zt}},lightmap:{lightMap:{value:null},lightMapIntensity:{value:1},lightMapTransform:{value:new zt}},bumpmap:{bumpMap:{value:null},bumpMapTransform:{value:new zt},bumpScale:{value:1}},normalmap:{normalMap:{value:null},normalMapTransform:{value:new zt},normalScale:{value:new yt(1,1)}},displacementmap:{displacementMap:{value:null},displacementMapTransform:{value:new zt},displacementScale:{value:1},displacementBias:{value:0}},emissivemap:{emissiveMap:{value:null},emissiveMapTransform:{value:new zt}},metalnessmap:{metalnessMap:{value:null},metalnessMapTransform:{value:new zt}},roughnessmap:{roughnessMap:{value:null},roughnessMapTransform:{value:new zt}},gradientmap:{gradientMap:{value:null}},fog:{fogDensity:{value:25e-5},fogNear:{value:1},fogFar:{value:2e3},fogColor:{value:new Ot(16777215)}},lights:{ambientLightColor:{value:[]},lightProbe:{value:[]},directionalLights:{value:[],properties:{direction:{},color:{}}},directionalLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},directionalShadowMap:{value:[]},directionalShadowMatrix:{value:[]},spotLights:{value:[],properties:{color:{},position:{},direction:{},distance:{},coneCos:{},penumbraCos:{},decay:{}}},spotLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},spotLightMap:{value:[]},spotShadowMap:{value:[]},spotLightMatrix:{value:[]},pointLights:{value:[],properties:{color:{},position:{},decay:{},distance:{}}},pointLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{},shadowCameraNear:{},shadowCameraFar:{}}},pointShadowMap:{value:[]},pointShadowMatrix:{value:[]},hemisphereLights:{value:[],properties:{direction:{},skyColor:{},groundColor:{}}},rectAreaLights:{value:[],properties:{color:{},position:{},width:{},height:{}}},ltc_1:{value:null},ltc_2:{value:null}},points:{diffuse:{value:new Ot(16777215)},opacity:{value:1},size:{value:1},scale:{value:1},map:{value:null},alphaMap:{value:null},alphaMapTransform:{value:new zt},alphaTest:{value:0},uvTransform:{value:new zt}},sprite:{diffuse:{value:new Ot(16777215)},opacity:{value:1},center:{value:new yt(.5,.5)},rotation:{value:0},map:{value:null},mapTransform:{value:new zt},alphaMap:{value:null},alphaMapTransform:{value:new zt},alphaTest:{value:0}}},gn={basic:{uniforms:Ie([dt.common,dt.specularmap,dt.envmap,dt.aomap,dt.lightmap,dt.fog]),vertexShader:Bt.meshbasic_vert,fragmentShader:Bt.meshbasic_frag},lambert:{uniforms:Ie([dt.common,dt.specularmap,dt.envmap,dt.aomap,dt.lightmap,dt.emissivemap,dt.bumpmap,dt.normalmap,dt.displacementmap,dt.fog,dt.lights,{emissive:{value:new Ot(0)}}]),vertexShader:Bt.meshlambert_vert,fragmentShader:Bt.meshlambert_frag},phong:{uniforms:Ie([dt.common,dt.specularmap,dt.envmap,dt.aomap,dt.lightmap,dt.emissivemap,dt.bumpmap,dt.normalmap,dt.displacementmap,dt.fog,dt.lights,{emissive:{value:new Ot(0)},specular:{value:new Ot(1118481)},shininess:{value:30}}]),vertexShader:Bt.meshphong_vert,fragmentShader:Bt.meshphong_frag},standard:{uniforms:Ie([dt.common,dt.envmap,dt.aomap,dt.lightmap,dt.emissivemap,dt.bumpmap,dt.normalmap,dt.displacementmap,dt.roughnessmap,dt.metalnessmap,dt.fog,dt.lights,{emissive:{value:new Ot(0)},roughness:{value:1},metalness:{value:0},envMapIntensity:{value:1}}]),vertexShader:Bt.meshphysical_vert,fragmentShader:Bt.meshphysical_frag},toon:{uniforms:Ie([dt.common,dt.aomap,dt.lightmap,dt.emissivemap,dt.bumpmap,dt.normalmap,dt.displacementmap,dt.gradientmap,dt.fog,dt.lights,{emissive:{value:new Ot(0)}}]),vertexShader:Bt.meshtoon_vert,fragmentShader:Bt.meshtoon_frag},matcap:{uniforms:Ie([dt.common,dt.bumpmap,dt.normalmap,dt.displacementmap,dt.fog,{matcap:{value:null}}]),vertexShader:Bt.meshmatcap_vert,fragmentShader:Bt.meshmatcap_frag},points:{uniforms:Ie([dt.points,dt.fog]),vertexShader:Bt.points_vert,fragmentShader:Bt.points_frag},dashed:{uniforms:Ie([dt.common,dt.fog,{scale:{value:1},dashSize:{value:1},totalSize:{value:2}}]),vertexShader:Bt.linedashed_vert,fragmentShader:Bt.linedashed_frag},depth:{uniforms:Ie([dt.common,dt.displacementmap]),vertexShader:Bt.depth_vert,fragmentShader:Bt.depth_frag},normal:{uniforms:Ie([dt.common,dt.bumpmap,dt.normalmap,dt.displacementmap,{opacity:{value:1}}]),vertexShader:Bt.meshnormal_vert,fragmentShader:Bt.meshnormal_frag},sprite:{uniforms:Ie([dt.sprite,dt.fog]),vertexShader:Bt.sprite_vert,fragmentShader:Bt.sprite_frag},background:{uniforms:{uvTransform:{value:new zt},t2D:{value:null},backgroundIntensity:{value:1}},vertexShader:Bt.background_vert,fragmentShader:Bt.background_frag},backgroundCube:{uniforms:{envMap:{value:null},flipEnvMap:{value:-1},backgroundBlurriness:{value:0},backgroundIntensity:{value:1},backgroundRotation:{value:new zt}},vertexShader:Bt.backgroundCube_vert,fragmentShader:Bt.backgroundCube_frag},cube:{uniforms:{tCube:{value:null},tFlip:{value:-1},opacity:{value:1}},vertexShader:Bt.cube_vert,fragmentShader:Bt.cube_frag},equirect:{uniforms:{tEquirect:{value:null}},vertexShader:Bt.equirect_vert,fragmentShader:Bt.equirect_frag},distanceRGBA:{uniforms:Ie([dt.common,dt.displacementmap,{referencePosition:{value:new U},nearDistance:{value:1},farDistance:{value:1e3}}]),vertexShader:Bt.distanceRGBA_vert,fragmentShader:Bt.distanceRGBA_frag},shadow:{uniforms:Ie([dt.lights,dt.fog,{color:{value:new Ot(0)},opacity:{value:1}}]),vertexShader:Bt.shadow_vert,fragmentShader:Bt.shadow_frag}};gn.physical={uniforms:Ie([gn.standard.uniforms,{clearcoat:{value:0},clearcoatMap:{value:null},clearcoatMapTransform:{value:new zt},clearcoatNormalMap:{value:null},clearcoatNormalMapTransform:{value:new zt},clearcoatNormalScale:{value:new yt(1,1)},clearcoatRoughness:{value:0},clearcoatRoughnessMap:{value:null},clearcoatRoughnessMapTransform:{value:new zt},dispersion:{value:0},iridescence:{value:0},iridescenceMap:{value:null},iridescenceMapTransform:{value:new zt},iridescenceIOR:{value:1.3},iridescenceThicknessMinimum:{value:100},iridescenceThicknessMaximum:{value:400},iridescenceThicknessMap:{value:null},iridescenceThicknessMapTransform:{value:new zt},sheen:{value:0},sheenColor:{value:new Ot(0)},sheenColorMap:{value:null},sheenColorMapTransform:{value:new zt},sheenRoughness:{value:1},sheenRoughnessMap:{value:null},sheenRoughnessMapTransform:{value:new zt},transmission:{value:0},transmissionMap:{value:null},transmissionMapTransform:{value:new zt},transmissionSamplerSize:{value:new yt},transmissionSamplerMap:{value:null},thickness:{value:0},thicknessMap:{value:null},thicknessMapTransform:{value:new zt},attenuationDistance:{value:0},attenuationColor:{value:new Ot(0)},specularColor:{value:new Ot(1,1,1)},specularColorMap:{value:null},specularColorMapTransform:{value:new zt},specularIntensity:{value:1},specularIntensityMap:{value:null},specularIntensityMapTransform:{value:new zt},anisotropyVector:{value:new yt},anisotropyMap:{value:null},anisotropyMapTransform:{value:new zt}}]),vertexShader:Bt.meshphysical_vert,fragmentShader:Bt.meshphysical_frag};var Br={r:0,b:0,g:0},hi=new tn,kg=new $t;function Hg(n,t,e,i,s,r,o){let a=new Ot(0),l=r===!0?0:1,h,f,d=null,c=0,u=null;function g(w){let _=w.isScene===!0?w.background:null;return _&&_.isTexture&&(_=(w.backgroundBlurriness>0?e:t).get(_)),_}function y(w){let _=!1,A=g(w);A===null?m(a,l):A&&A.isColor&&(m(A,1),_=!0);let I=n.xr.getEnvironmentBlendMode();I==="additive"?i.buffers.color.setClear(0,0,0,1,o):I==="alpha-blend"&&i.buffers.color.setClear(0,0,0,0,o),(n.autoClear||_)&&(i.buffers.depth.setTest(!0),i.buffers.depth.setMask(!0),i.buffers.color.setMask(!0),n.clear(n.autoClearColor,n.autoClearDepth,n.autoClearStencil))}function p(w,_){let A=g(_);A&&(A.isCubeTexture||A.mapping===wo)?(f===void 0&&(f=new Ue(new er(1,1,1),new ie({name:"BackgroundCubeMaterial",uniforms:ps(gn.backgroundCube.uniforms),vertexShader:gn.backgroundCube.vertexShader,fragmentShader:gn.backgroundCube.fragmentShader,side:Fe,depthTest:!1,depthWrite:!1,fog:!1})),f.geometry.deleteAttribute("normal"),f.geometry.deleteAttribute("uv"),f.onBeforeRender=function(I,C,E){this.matrixWorld.copyPosition(E.matrixWorld)},Object.defineProperty(f.material,"envMap",{get:function(){return this.uniforms.envMap.value}}),s.update(f)),hi.copy(_.backgroundRotation),hi.x*=-1,hi.y*=-1,hi.z*=-1,A.isCubeTexture&&A.isRenderTargetTexture===!1&&(hi.y*=-1,hi.z*=-1),f.material.uniforms.envMap.value=A,f.material.uniforms.flipEnvMap.value=A.isCubeTexture&&A.isRenderTargetTexture===!1?-1:1,f.material.uniforms.backgroundBlurriness.value=_.backgroundBlurriness,f.material.uniforms.backgroundIntensity.value=_.backgroundIntensity,f.material.uniforms.backgroundRotation.value.setFromMatrix4(kg.makeRotationFromEuler(hi)),f.material.toneMapped=Jt.getTransfer(A.colorSpace)!==ee,(d!==A||c!==A.version||u!==n.toneMapping)&&(f.material.needsUpdate=!0,d=A,c=A.version,u=n.toneMapping),f.layers.enableAll(),w.unshift(f,f.geometry,f.material,0,0,null)):A&&A.isTexture&&(h===void 0&&(h=new Ue(new lo(2,2),new ie({name:"BackgroundMaterial",uniforms:ps(gn.background.uniforms),vertexShader:gn.background.vertexShader,fragmentShader:gn.background.fragmentShader,side:jn,depthTest:!1,depthWrite:!1,fog:!1})),h.geometry.deleteAttribute("normal"),Object.defineProperty(h.material,"map",{get:function(){return this.uniforms.t2D.value}}),s.update(h)),h.material.uniforms.t2D.value=A,h.material.uniforms.backgroundIntensity.value=_.backgroundIntensity,h.material.toneMapped=Jt.getTransfer(A.colorSpace)!==ee,A.matrixAutoUpdate===!0&&A.updateMatrix(),h.material.uniforms.uvTransform.value.copy(A.matrix),(d!==A||c!==A.version||u!==n.toneMapping)&&(h.material.needsUpdate=!0,d=A,c=A.version,u=n.toneMapping),h.layers.enableAll(),w.unshift(h,h.geometry,h.material,0,0,null))}function m(w,_){w.getRGB(Br,wu(n)),i.buffers.color.setClear(Br.r,Br.g,Br.b,_,o)}return{getClearColor:function(){return a},setClearColor:function(w,_=1){a.set(w),l=_,m(a,l)},getClearAlpha:function(){return l},setClearAlpha:function(w){l=w,m(a,l)},render:y,addToRenderList:p}}function Vg(n,t){let e=n.getParameter(n.MAX_VERTEX_ATTRIBS),i={},s=c(null),r=s,o=!1;function a(x,S,O,W,q){let et=!1,F=d(W,O,S);r!==F&&(r=F,h(r.object)),et=u(x,W,O,q),et&&g(x,W,O,q),q!==null&&t.update(q,n.ELEMENT_ARRAY_BUFFER),(et||o)&&(o=!1,A(x,S,O,W),q!==null&&n.bindBuffer(n.ELEMENT_ARRAY_BUFFER,t.get(q).buffer))}function l(){return n.createVertexArray()}function h(x){return n.bindVertexArray(x)}function f(x){return n.deleteVertexArray(x)}function d(x,S,O){let W=O.wireframe===!0,q=i[x.id];q===void 0&&(q={},i[x.id]=q);let et=q[S.id];et===void 0&&(et={},q[S.id]=et);let F=et[W];return F===void 0&&(F=c(l()),et[W]=F),F}function c(x){let S=[],O=[],W=[];for(let q=0;q<e;q++)S[q]=0,O[q]=0,W[q]=0;return{geometry:null,program:null,wireframe:!1,newAttributes:S,enabledAttributes:O,attributeDivisors:W,object:x,attributes:{},index:null}}function u(x,S,O,W){let q=r.attributes,et=S.attributes,F=0,it=O.getAttributes();for(let Z in it)if(it[Z].location>=0){let _t=q[Z],St=et[Z];if(St===void 0&&(Z==="instanceMatrix"&&x.instanceMatrix&&(St=x.instanceMatrix),Z==="instanceColor"&&x.instanceColor&&(St=x.instanceColor)),_t===void 0||_t.attribute!==St||St&&_t.data!==St.data)return!0;F++}return r.attributesNum!==F||r.index!==W}function g(x,S,O,W){let q={},et=S.attributes,F=0,it=O.getAttributes();for(let Z in it)if(it[Z].location>=0){let _t=et[Z];_t===void 0&&(Z==="instanceMatrix"&&x.instanceMatrix&&(_t=x.instanceMatrix),Z==="instanceColor"&&x.instanceColor&&(_t=x.instanceColor));let St={};St.attribute=_t,_t&&_t.data&&(St.data=_t.data),q[Z]=St,F++}r.attributes=q,r.attributesNum=F,r.index=W}function y(){let x=r.newAttributes;for(let S=0,O=x.length;S<O;S++)x[S]=0}function p(x){m(x,0)}function m(x,S){let O=r.newAttributes,W=r.enabledAttributes,q=r.attributeDivisors;O[x]=1,W[x]===0&&(n.enableVertexAttribArray(x),W[x]=1),q[x]!==S&&(n.vertexAttribDivisor(x,S),q[x]=S)}function w(){let x=r.newAttributes,S=r.enabledAttributes;for(let O=0,W=S.length;O<W;O++)S[O]!==x[O]&&(n.disableVertexAttribArray(O),S[O]=0)}function _(x,S,O,W,q,et,F){F===!0?n.vertexAttribIPointer(x,S,O,q,et):n.vertexAttribPointer(x,S,O,W,q,et)}function A(x,S,O,W){y();let q=W.attributes,et=O.getAttributes(),F=S.defaultAttributeValues;for(let it in et){let Z=et[it];if(Z.location>=0){let gt=q[it];if(gt===void 0&&(it==="instanceMatrix"&&x.instanceMatrix&&(gt=x.instanceMatrix),it==="instanceColor"&&x.instanceColor&&(gt=x.instanceColor)),gt!==void 0){let _t=gt.normalized,St=gt.itemSize,Gt=t.get(gt);if(Gt===void 0)continue;let Wt=Gt.buffer,J=Gt.type,st=Gt.bytesPerElement,Mt=J===n.INT||J===n.UNSIGNED_INT||gt.gpuType===rc;if(gt.isInterleavedBufferAttribute){let xt=gt.data,Nt=xt.stride,It=gt.offset;if(xt.isInstancedInterleavedBuffer){for(let kt=0;kt<Z.locationSize;kt++)m(Z.location+kt,xt.meshPerAttribute);x.isInstancedMesh!==!0&&W._maxInstanceCount===void 0&&(W._maxInstanceCount=xt.meshPerAttribute*xt.count)}else for(let kt=0;kt<Z.locationSize;kt++)p(Z.location+kt);n.bindBuffer(n.ARRAY_BUFFER,Wt);for(let kt=0;kt<Z.locationSize;kt++)_(Z.location+kt,St/Z.locationSize,J,_t,Nt*st,(It+St/Z.locationSize*kt)*st,Mt)}else{if(gt.isInstancedBufferAttribute){for(let xt=0;xt<Z.locationSize;xt++)m(Z.location+xt,gt.meshPerAttribute);x.isInstancedMesh!==!0&&W._maxInstanceCount===void 0&&(W._maxInstanceCount=gt.meshPerAttribute*gt.count)}else for(let xt=0;xt<Z.locationSize;xt++)p(Z.location+xt);n.bindBuffer(n.ARRAY_BUFFER,Wt);for(let xt=0;xt<Z.locationSize;xt++)_(Z.location+xt,St/Z.locationSize,J,_t,St*st,St/Z.locationSize*xt*st,Mt)}}else if(F!==void 0){let _t=F[it];if(_t!==void 0)switch(_t.length){case 2:n.vertexAttrib2fv(Z.location,_t);break;case 3:n.vertexAttrib3fv(Z.location,_t);break;case 4:n.vertexAttrib4fv(Z.location,_t);break;default:n.vertexAttrib1fv(Z.location,_t)}}}}w()}function I(){T();for(let x in i){let S=i[x];for(let O in S){let W=S[O];for(let q in W)f(W[q].object),delete W[q];delete S[O]}delete i[x]}}function C(x){if(i[x.id]===void 0)return;let S=i[x.id];for(let O in S){let W=S[O];for(let q in W)f(W[q].object),delete W[q];delete S[O]}delete i[x.id]}function E(x){for(let S in i){let O=i[S];if(O[x.id]===void 0)continue;let W=O[x.id];for(let q in W)f(W[q].object),delete W[q];delete O[x.id]}}function T(){H(),o=!0,r!==s&&(r=s,h(r.object))}function H(){s.geometry=null,s.program=null,s.wireframe=!1}return{setup:a,reset:T,resetDefaultState:H,dispose:I,releaseStatesOfGeometry:C,releaseStatesOfProgram:E,initAttributes:y,enableAttribute:p,disableUnusedAttributes:w}}function Gg(n,t,e){let i;function s(h){i=h}function r(h,f){n.drawArrays(i,h,f),e.update(f,i,1)}function o(h,f,d){d!==0&&(n.drawArraysInstanced(i,h,f,d),e.update(f,i,d))}function a(h,f,d){if(d===0)return;t.get("WEBGL_multi_draw").multiDrawArraysWEBGL(i,h,0,f,0,d);let u=0;for(let g=0;g<d;g++)u+=f[g];e.update(u,i,1)}function l(h,f,d,c){if(d===0)return;let u=t.get("WEBGL_multi_draw");if(u===null)for(let g=0;g<h.length;g++)o(h[g],f[g],c[g]);else{u.multiDrawArraysInstancedWEBGL(i,h,0,f,0,c,0,d);let g=0;for(let y=0;y<d;y++)g+=f[y];for(let y=0;y<c.length;y++)e.update(g,i,c[y])}}this.setMode=s,this.render=r,this.renderInstances=o,this.renderMultiDraw=a,this.renderMultiDrawInstances=l}function Wg(n,t,e,i){let s;function r(){if(s!==void 0)return s;if(t.has("EXT_texture_filter_anisotropic")===!0){let E=t.get("EXT_texture_filter_anisotropic");s=n.getParameter(E.MAX_TEXTURE_MAX_ANISOTROPY_EXT)}else s=0;return s}function o(E){return!(E!==hn&&i.convert(E)!==n.getParameter(n.IMPLEMENTATION_COLOR_READ_FORMAT))}function a(E){let T=E===ze&&(t.has("EXT_color_buffer_half_float")||t.has("EXT_color_buffer_float"));return!(E!==In&&i.convert(E)!==n.getParameter(n.IMPLEMENTATION_COLOR_READ_TYPE)&&E!==xn&&!T)}function l(E){if(E==="highp"){if(n.getShaderPrecisionFormat(n.VERTEX_SHADER,n.HIGH_FLOAT).precision>0&&n.getShaderPrecisionFormat(n.FRAGMENT_SHADER,n.HIGH_FLOAT).precision>0)return"highp";E="mediump"}return E==="mediump"&&n.getShaderPrecisionFormat(n.VERTEX_SHADER,n.MEDIUM_FLOAT).precision>0&&n.getShaderPrecisionFormat(n.FRAGMENT_SHADER,n.MEDIUM_FLOAT).precision>0?"mediump":"lowp"}let h=e.precision!==void 0?e.precision:"highp",f=l(h);f!==h&&(console.warn("THREE.WebGLRenderer:",h,"not supported, using",f,"instead."),h=f);let d=e.logarithmicDepthBuffer===!0,c=e.reverseDepthBuffer===!0&&t.has("EXT_clip_control");if(c===!0){let E=t.get("EXT_clip_control");E.clipControlEXT(E.LOWER_LEFT_EXT,E.ZERO_TO_ONE_EXT)}let u=n.getParameter(n.MAX_TEXTURE_IMAGE_UNITS),g=n.getParameter(n.MAX_VERTEX_TEXTURE_IMAGE_UNITS),y=n.getParameter(n.MAX_TEXTURE_SIZE),p=n.getParameter(n.MAX_CUBE_MAP_TEXTURE_SIZE),m=n.getParameter(n.MAX_VERTEX_ATTRIBS),w=n.getParameter(n.MAX_VERTEX_UNIFORM_VECTORS),_=n.getParameter(n.MAX_VARYING_VECTORS),A=n.getParameter(n.MAX_FRAGMENT_UNIFORM_VECTORS),I=g>0,C=n.getParameter(n.MAX_SAMPLES);return{isWebGL2:!0,getMaxAnisotropy:r,getMaxPrecision:l,textureFormatReadable:o,textureTypeReadable:a,precision:h,logarithmicDepthBuffer:d,reverseDepthBuffer:c,maxTextures:u,maxVertexTextures:g,maxTextureSize:y,maxCubemapSize:p,maxAttributes:m,maxVertexUniforms:w,maxVaryings:_,maxFragmentUniforms:A,vertexTextures:I,maxSamples:C}}function Xg(n){let t=this,e=null,i=0,s=!1,r=!1,o=new cn,a=new zt,l={value:null,needsUpdate:!1};this.uniform=l,this.numPlanes=0,this.numIntersection=0,this.init=function(d,c){let u=d.length!==0||c||i!==0||s;return s=c,i=d.length,u},this.beginShadows=function(){r=!0,f(null)},this.endShadows=function(){r=!1},this.setGlobalState=function(d,c){e=f(d,c,0)},this.setState=function(d,c,u){let g=d.clippingPlanes,y=d.clipIntersection,p=d.clipShadows,m=n.get(d);if(!s||g===null||g.length===0||r&&!p)r?f(null):h();else{let w=r?0:i,_=w*4,A=m.clippingState||null;l.value=A,A=f(g,c,_,u);for(let I=0;I!==_;++I)A[I]=e[I];m.clippingState=A,this.numIntersection=y?this.numPlanes:0,this.numPlanes+=w}};function h(){l.value!==e&&(l.value=e,l.needsUpdate=i>0),t.numPlanes=i,t.numIntersection=0}function f(d,c,u,g){let y=d!==null?d.length:0,p=null;if(y!==0){if(p=l.value,g!==!0||p===null){let m=u+y*4,w=c.matrixWorldInverse;a.getNormalMatrix(w),(p===null||p.length<m)&&(p=new Float32Array(m));for(let _=0,A=u;_!==y;++_,A+=4)o.copy(d[_]).applyMatrix4(w,a),o.normal.toArray(p,A),p[A+3]=o.constant}l.value=p,l.needsUpdate=!0}return t.numPlanes=y,t.numIntersection=0,p}}function qg(n){let t=new WeakMap;function e(o,a){return a===Ja?o.mapping=cs:a===Qa&&(o.mapping=hs),o}function i(o){if(o&&o.isTexture){let a=o.mapping;if(a===Ja||a===Qa)if(t.has(o)){let l=t.get(o).texture;return e(l,o.mapping)}else{let l=o.image;if(l&&l.height>0){let h=new Ll(l.height);return h.fromEquirectangularTexture(n,o),t.set(o,h),o.addEventListener("dispose",s),e(h.texture,o.mapping)}else return null}}return o}function s(o){let a=o.target;a.removeEventListener("dispose",s);let l=t.get(a);l!==void 0&&(t.delete(a),l.dispose())}function r(){t=new WeakMap}return{get:i,dispose:r}}var ms=class extends oo{constructor(t=-1,e=1,i=1,s=-1,r=.1,o=2e3){super(),this.isOrthographicCamera=!0,this.type="OrthographicCamera",this.zoom=1,this.view=null,this.left=t,this.right=e,this.top=i,this.bottom=s,this.near=r,this.far=o,this.updateProjectionMatrix()}copy(t,e){return super.copy(t,e),this.left=t.left,this.right=t.right,this.top=t.top,this.bottom=t.bottom,this.near=t.near,this.far=t.far,this.zoom=t.zoom,this.view=t.view===null?null:Object.assign({},t.view),this}setViewOffset(t,e,i,s,r,o){this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=t,this.view.fullHeight=e,this.view.offsetX=i,this.view.offsetY=s,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let t=(this.right-this.left)/(2*this.zoom),e=(this.top-this.bottom)/(2*this.zoom),i=(this.right+this.left)/2,s=(this.top+this.bottom)/2,r=i-t,o=i+t,a=s+e,l=s-e;if(this.view!==null&&this.view.enabled){let h=(this.right-this.left)/this.view.fullWidth/this.zoom,f=(this.top-this.bottom)/this.view.fullHeight/this.zoom;r+=h*this.view.offsetX,o=r+h*this.view.width,a-=f*this.view.offsetY,l=a-f*this.view.height}this.projectionMatrix.makeOrthographic(r,o,a,l,this.near,this.far,this.coordinateSystem),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(t){let e=super.toJSON(t);return e.object.zoom=this.zoom,e.object.left=this.left,e.object.right=this.right,e.object.top=this.top,e.object.bottom=this.bottom,e.object.near=this.near,e.object.far=this.far,this.view!==null&&(e.object.view=Object.assign({},this.view)),e}},ns=4,Lh=[.125,.215,.35,.446,.526,.582],mi=20,Na=new ms,Dh=new Ot,Oa=null,Fa=0,Ba=0,za=!1,di=(1+Math.sqrt(5))/2,$i=1/di,Uh=[new U(-di,$i,0),new U(di,$i,0),new U(-$i,0,di),new U($i,0,di),new U(0,di,-$i),new U(0,di,$i),new U(-1,1,-1),new U(1,1,-1),new U(-1,1,1),new U(1,1,1)],co=class{constructor(t){this._renderer=t,this._pingPongRenderTarget=null,this._lodMax=0,this._cubeSize=0,this._lodPlanes=[],this._sizeLods=[],this._sigmas=[],this._blurMaterial=null,this._cubemapMaterial=null,this._equirectMaterial=null,this._compileMaterial(this._blurMaterial)}fromScene(t,e=0,i=.1,s=100){Oa=this._renderer.getRenderTarget(),Fa=this._renderer.getActiveCubeFace(),Ba=this._renderer.getActiveMipmapLevel(),za=this._renderer.xr.enabled,this._renderer.xr.enabled=!1,this._setSize(256);let r=this._allocateTargets();return r.depthBuffer=!0,this._sceneToCubeUV(t,i,s,r),e>0&&this._blur(r,0,0,e),this._applyPMREM(r),this._cleanup(r),r}fromEquirectangular(t,e=null){return this._fromTexture(t,e)}fromCubemap(t,e=null){return this._fromTexture(t,e)}compileCubemapShader(){this._cubemapMaterial===null&&(this._cubemapMaterial=Fh(),this._compileMaterial(this._cubemapMaterial))}compileEquirectangularShader(){this._equirectMaterial===null&&(this._equirectMaterial=Oh(),this._compileMaterial(this._equirectMaterial))}dispose(){this._dispose(),this._cubemapMaterial!==null&&this._cubemapMaterial.dispose(),this._equirectMaterial!==null&&this._equirectMaterial.dispose()}_setSize(t){this._lodMax=Math.floor(Math.log2(t)),this._cubeSize=Math.pow(2,this._lodMax)}_dispose(){this._blurMaterial!==null&&this._blurMaterial.dispose(),this._pingPongRenderTarget!==null&&this._pingPongRenderTarget.dispose();for(let t=0;t<this._lodPlanes.length;t++)this._lodPlanes[t].dispose()}_cleanup(t){this._renderer.setRenderTarget(Oa,Fa,Ba),this._renderer.xr.enabled=za,t.scissorTest=!1,zr(t,0,0,t.width,t.height)}_fromTexture(t,e){t.mapping===cs||t.mapping===hs?this._setSize(t.image.length===0?16:t.image[0].width||t.image[0].image.width):this._setSize(t.image.width/4),Oa=this._renderer.getRenderTarget(),Fa=this._renderer.getActiveCubeFace(),Ba=this._renderer.getActiveMipmapLevel(),za=this._renderer.xr.enabled,this._renderer.xr.enabled=!1;let i=e||this._allocateTargets();return this._textureToCubeUV(t,i),this._applyPMREM(i),this._cleanup(i),i}_allocateTargets(){let t=3*Math.max(this._cubeSize,112),e=4*this._cubeSize,i={magFilter:Xe,minFilter:Xe,generateMipmaps:!1,type:ze,format:hn,colorSpace:Kn,depthBuffer:!1},s=Nh(t,e,i);if(this._pingPongRenderTarget===null||this._pingPongRenderTarget.width!==t||this._pingPongRenderTarget.height!==e){this._pingPongRenderTarget!==null&&this._dispose(),this._pingPongRenderTarget=Nh(t,e,i);let{_lodMax:r}=this;({sizeLods:this._sizeLods,lodPlanes:this._lodPlanes,sigmas:this._sigmas}=Yg(r)),this._blurMaterial=Zg(r,t,e)}return s}_compileMaterial(t){let e=new Ue(this._lodPlanes[0],t);this._renderer.compile(e,Na)}_sceneToCubeUV(t,e,i,s){let a=new De(90,1,e,i),l=[1,-1,1,1,1,1],h=[1,1,1,-1,-1,-1],f=this._renderer,d=f.autoClear,c=f.toneMapping;f.getClearColor(Dh),f.toneMapping=Zn,f.autoClear=!1;let u=new fs({name:"PMREM.Background",side:Fe,depthWrite:!1,depthTest:!1}),g=new Ue(new er,u),y=!1,p=t.background;p?p.isColor&&(u.color.copy(p),t.background=null,y=!0):(u.color.copy(Dh),y=!0);for(let m=0;m<6;m++){let w=m%3;w===0?(a.up.set(0,l[m],0),a.lookAt(h[m],0,0)):w===1?(a.up.set(0,0,l[m]),a.lookAt(0,h[m],0)):(a.up.set(0,l[m],0),a.lookAt(0,0,h[m]));let _=this._cubeSize;zr(s,w*_,m>2?_:0,_,_),f.setRenderTarget(s),y&&f.render(g,a),f.render(t,a)}g.geometry.dispose(),g.material.dispose(),f.toneMapping=c,f.autoClear=d,t.background=p}_textureToCubeUV(t,e){let i=this._renderer,s=t.mapping===cs||t.mapping===hs;s?(this._cubemapMaterial===null&&(this._cubemapMaterial=Fh()),this._cubemapMaterial.uniforms.flipEnvMap.value=t.isRenderTargetTexture===!1?-1:1):this._equirectMaterial===null&&(this._equirectMaterial=Oh());let r=s?this._cubemapMaterial:this._equirectMaterial,o=new Ue(this._lodPlanes[0],r),a=r.uniforms;a.envMap.value=t;let l=this._cubeSize;zr(e,0,0,3*l,2*l),i.setRenderTarget(e),i.render(o,Na)}_applyPMREM(t){let e=this._renderer,i=e.autoClear;e.autoClear=!1;let s=this._lodPlanes.length;for(let r=1;r<s;r++){let o=Math.sqrt(this._sigmas[r]*this._sigmas[r]-this._sigmas[r-1]*this._sigmas[r-1]),a=Uh[(s-r-1)%Uh.length];this._blur(t,r-1,r,o,a)}e.autoClear=i}_blur(t,e,i,s,r){let o=this._pingPongRenderTarget;this._halfBlur(t,o,e,i,s,"latitudinal",r),this._halfBlur(o,t,i,i,s,"longitudinal",r)}_halfBlur(t,e,i,s,r,o,a){let l=this._renderer,h=this._blurMaterial;o!=="latitudinal"&&o!=="longitudinal"&&console.error("blur direction must be either latitudinal or longitudinal!");let f=3,d=new Ue(this._lodPlanes[s],h),c=h.uniforms,u=this._sizeLods[i]-1,g=isFinite(r)?Math.PI/(2*u):2*Math.PI/(2*mi-1),y=r/g,p=isFinite(r)?1+Math.floor(f*y):mi;p>mi&&console.warn(`sigmaRadians, ${r}, is too large and will clip, as it requested ${p} samples when the maximum is set to ${mi}`);let m=[],w=0;for(let E=0;E<mi;++E){let T=E/y,H=Math.exp(-T*T/2);m.push(H),E===0?w+=H:E<p&&(w+=2*H)}for(let E=0;E<m.length;E++)m[E]=m[E]/w;c.envMap.value=t.texture,c.samples.value=p,c.weights.value=m,c.latitudinal.value=o==="latitudinal",a&&(c.poleAxis.value=a);let{_lodMax:_}=this;c.dTheta.value=g,c.mipInt.value=_-i;let A=this._sizeLods[s],I=3*A*(s>_-ns?s-_+ns:0),C=4*(this._cubeSize-A);zr(e,I,C,3*A,2*A),l.setRenderTarget(e),l.render(d,Na)}};function Yg(n){let t=[],e=[],i=[],s=n,r=n-ns+1+Lh.length;for(let o=0;o<r;o++){let a=Math.pow(2,s);e.push(a);let l=1/a;o>n-ns?l=Lh[o-n+ns-1]:o===0&&(l=0),i.push(l);let h=1/(a-2),f=-h,d=1+h,c=[f,f,d,f,d,d,f,f,d,d,f,d],u=6,g=6,y=3,p=2,m=1,w=new Float32Array(y*g*u),_=new Float32Array(p*g*u),A=new Float32Array(m*g*u);for(let C=0;C<u;C++){let E=C%3*2/3-1,T=C>2?0:-1,H=[E,T,0,E+2/3,T,0,E+2/3,T+1,0,E,T,0,E+2/3,T+1,0,E,T+1,0];w.set(H,y*g*C),_.set(c,p*g*C);let x=[C,C,C,C,C,C];A.set(x,m*g*C)}let I=new un;I.setAttribute("position",new qe(w,y)),I.setAttribute("uv",new qe(_,p)),I.setAttribute("faceIndex",new qe(A,m)),t.push(I),s>ns&&s--}return{lodPlanes:t,sizeLods:e,sigmas:i}}function Nh(n,t,e){let i=new ue(n,t,e);return i.texture.mapping=wo,i.texture.name="PMREM.cubeUv",i.scissorTest=!0,i}function zr(n,t,e,i,s){n.viewport.set(t,e,i,s),n.scissor.set(t,e,i,s)}function Zg(n,t,e){let i=new Float32Array(mi),s=new U(0,1,0);return new ie({name:"SphericalGaussianBlur",defines:{n:mi,CUBEUV_TEXEL_WIDTH:1/t,CUBEUV_TEXEL_HEIGHT:1/e,CUBEUV_MAX_MIP:`${n}.0`},uniforms:{envMap:{value:null},samples:{value:1},weights:{value:i},latitudinal:{value:!1},dTheta:{value:0},mipInt:{value:0},poleAxis:{value:s}},vertexShader:mc(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			varying vec3 vOutputDirection;

			uniform sampler2D envMap;
			uniform int samples;
			uniform float weights[ n ];
			uniform bool latitudinal;
			uniform float dTheta;
			uniform float mipInt;
			uniform vec3 poleAxis;

			#define ENVMAP_TYPE_CUBE_UV
			#include <cube_uv_reflection_fragment>

			vec3 getSample( float theta, vec3 axis ) {

				float cosTheta = cos( theta );
				// Rodrigues' axis-angle rotation
				vec3 sampleDirection = vOutputDirection * cosTheta
					+ cross( axis, vOutputDirection ) * sin( theta )
					+ axis * dot( axis, vOutputDirection ) * ( 1.0 - cosTheta );

				return bilinearCubeUV( envMap, sampleDirection, mipInt );

			}

			void main() {

				vec3 axis = latitudinal ? poleAxis : cross( poleAxis, vOutputDirection );

				if ( all( equal( axis, vec3( 0.0 ) ) ) ) {

					axis = vec3( vOutputDirection.z, 0.0, - vOutputDirection.x );

				}

				axis = normalize( axis );

				gl_FragColor = vec4( 0.0, 0.0, 0.0, 1.0 );
				gl_FragColor.rgb += weights[ 0 ] * getSample( 0.0, axis );

				for ( int i = 1; i < n; i++ ) {

					if ( i >= samples ) {

						break;

					}

					float theta = dTheta * float( i );
					gl_FragColor.rgb += weights[ i ] * getSample( -1.0 * theta, axis );
					gl_FragColor.rgb += weights[ i ] * getSample( theta, axis );

				}

			}
		`,blending:vn,depthTest:!1,depthWrite:!1})}function Oh(){return new ie({name:"EquirectangularToCubeUV",uniforms:{envMap:{value:null}},vertexShader:mc(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			varying vec3 vOutputDirection;

			uniform sampler2D envMap;

			#include <common>

			void main() {

				vec3 outputDirection = normalize( vOutputDirection );
				vec2 uv = equirectUv( outputDirection );

				gl_FragColor = vec4( texture2D ( envMap, uv ).rgb, 1.0 );

			}
		`,blending:vn,depthTest:!1,depthWrite:!1})}function Fh(){return new ie({name:"CubemapToCubeUV",uniforms:{envMap:{value:null},flipEnvMap:{value:-1}},vertexShader:mc(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			uniform float flipEnvMap;

			varying vec3 vOutputDirection;

			uniform samplerCube envMap;

			void main() {

				gl_FragColor = textureCube( envMap, vec3( flipEnvMap * vOutputDirection.x, vOutputDirection.yz ) );

			}
		`,blending:vn,depthTest:!1,depthWrite:!1})}function mc(){return`

		precision mediump float;
		precision mediump int;

		attribute float faceIndex;

		varying vec3 vOutputDirection;

		// RH coordinate system; PMREM face-indexing convention
		vec3 getDirection( vec2 uv, float face ) {

			uv = 2.0 * uv - 1.0;

			vec3 direction = vec3( uv, 1.0 );

			if ( face == 0.0 ) {

				direction = direction.zyx; // ( 1, v, u ) pos x

			} else if ( face == 1.0 ) {

				direction = direction.xzy;
				direction.xz *= -1.0; // ( -u, 1, -v ) pos y

			} else if ( face == 2.0 ) {

				direction.x *= -1.0; // ( -u, v, 1 ) pos z

			} else if ( face == 3.0 ) {

				direction = direction.zyx;
				direction.xz *= -1.0; // ( -1, v, -u ) neg x

			} else if ( face == 4.0 ) {

				direction = direction.xzy;
				direction.xy *= -1.0; // ( -u, -1, v ) neg y

			} else if ( face == 5.0 ) {

				direction.z *= -1.0; // ( u, v, -1 ) neg z

			}

			return direction;

		}

		void main() {

			vOutputDirection = getDirection( uv, faceIndex );
			gl_Position = vec4( position, 1.0 );

		}
	`}function jg(n){let t=new WeakMap,e=null;function i(a){if(a&&a.isTexture){let l=a.mapping,h=l===Ja||l===Qa,f=l===cs||l===hs;if(h||f){let d=t.get(a),c=d!==void 0?d.texture.pmremVersion:0;if(a.isRenderTargetTexture&&a.pmremVersion!==c)return e===null&&(e=new co(n)),d=h?e.fromEquirectangular(a,d):e.fromCubemap(a,d),d.texture.pmremVersion=a.pmremVersion,t.set(a,d),d.texture;if(d!==void 0)return d.texture;{let u=a.image;return h&&u&&u.height>0||f&&u&&s(u)?(e===null&&(e=new co(n)),d=h?e.fromEquirectangular(a):e.fromCubemap(a),d.texture.pmremVersion=a.pmremVersion,t.set(a,d),a.addEventListener("dispose",r),d.texture):null}}}return a}function s(a){let l=0,h=6;for(let f=0;f<h;f++)a[f]!==void 0&&l++;return l===h}function r(a){let l=a.target;l.removeEventListener("dispose",r);let h=t.get(l);h!==void 0&&(t.delete(l),h.dispose())}function o(){t=new WeakMap,e!==null&&(e.dispose(),e=null)}return{get:i,dispose:o}}function Kg(n){let t={};function e(i){if(t[i]!==void 0)return t[i];let s;switch(i){case"WEBGL_depth_texture":s=n.getExtension("WEBGL_depth_texture")||n.getExtension("MOZ_WEBGL_depth_texture")||n.getExtension("WEBKIT_WEBGL_depth_texture");break;case"EXT_texture_filter_anisotropic":s=n.getExtension("EXT_texture_filter_anisotropic")||n.getExtension("MOZ_EXT_texture_filter_anisotropic")||n.getExtension("WEBKIT_EXT_texture_filter_anisotropic");break;case"WEBGL_compressed_texture_s3tc":s=n.getExtension("WEBGL_compressed_texture_s3tc")||n.getExtension("MOZ_WEBGL_compressed_texture_s3tc")||n.getExtension("WEBKIT_WEBGL_compressed_texture_s3tc");break;case"WEBGL_compressed_texture_pvrtc":s=n.getExtension("WEBGL_compressed_texture_pvrtc")||n.getExtension("WEBKIT_WEBGL_compressed_texture_pvrtc");break;default:s=n.getExtension(i)}return t[i]=s,s}return{has:function(i){return e(i)!==null},init:function(){e("EXT_color_buffer_float"),e("WEBGL_clip_cull_distance"),e("OES_texture_float_linear"),e("EXT_color_buffer_half_float"),e("WEBGL_multisampled_render_to_texture"),e("WEBGL_render_shared_exponent")},get:function(i){let s=e(i);return s===null&&Zr("THREE.WebGLRenderer: "+i+" extension not supported."),s}}}function Jg(n,t,e,i){let s={},r=new WeakMap;function o(d){let c=d.target;c.index!==null&&t.remove(c.index);for(let g in c.attributes)t.remove(c.attributes[g]);for(let g in c.morphAttributes){let y=c.morphAttributes[g];for(let p=0,m=y.length;p<m;p++)t.remove(y[p])}c.removeEventListener("dispose",o),delete s[c.id];let u=r.get(c);u&&(t.remove(u),r.delete(c)),i.releaseStatesOfGeometry(c),c.isInstancedBufferGeometry===!0&&delete c._maxInstanceCount,e.memory.geometries--}function a(d,c){return s[c.id]===!0||(c.addEventListener("dispose",o),s[c.id]=!0,e.memory.geometries++),c}function l(d){let c=d.attributes;for(let g in c)t.update(c[g],n.ARRAY_BUFFER);let u=d.morphAttributes;for(let g in u){let y=u[g];for(let p=0,m=y.length;p<m;p++)t.update(y[p],n.ARRAY_BUFFER)}}function h(d){let c=[],u=d.index,g=d.attributes.position,y=0;if(u!==null){let w=u.array;y=u.version;for(let _=0,A=w.length;_<A;_+=3){let I=w[_+0],C=w[_+1],E=w[_+2];c.push(I,C,C,E,E,I)}}else if(g!==void 0){let w=g.array;y=g.version;for(let _=0,A=w.length/3-1;_<A;_+=3){let I=_+0,C=_+1,E=_+2;c.push(I,C,C,E,E,I)}}else return;let p=new(Su(c)?ro:so)(c,1);p.version=y;let m=r.get(d);m&&t.remove(m),r.set(d,p)}function f(d){let c=r.get(d);if(c){let u=d.index;u!==null&&c.version<u.version&&h(d)}else h(d);return r.get(d)}return{get:a,update:l,getWireframeAttribute:f}}function Qg(n,t,e){let i;function s(c){i=c}let r,o;function a(c){r=c.type,o=c.bytesPerElement}function l(c,u){n.drawElements(i,u,r,c*o),e.update(u,i,1)}function h(c,u,g){g!==0&&(n.drawElementsInstanced(i,u,r,c*o,g),e.update(u,i,g))}function f(c,u,g){if(g===0)return;t.get("WEBGL_multi_draw").multiDrawElementsWEBGL(i,u,0,r,c,0,g);let p=0;for(let m=0;m<g;m++)p+=u[m];e.update(p,i,1)}function d(c,u,g,y){if(g===0)return;let p=t.get("WEBGL_multi_draw");if(p===null)for(let m=0;m<c.length;m++)h(c[m]/o,u[m],y[m]);else{p.multiDrawElementsInstancedWEBGL(i,u,0,r,c,0,y,0,g);let m=0;for(let w=0;w<g;w++)m+=u[w];for(let w=0;w<y.length;w++)e.update(m,i,y[w])}}this.setMode=s,this.setIndex=a,this.render=l,this.renderInstances=h,this.renderMultiDraw=f,this.renderMultiDrawInstances=d}function $g(n){let t={geometries:0,textures:0},e={frame:0,calls:0,triangles:0,points:0,lines:0};function i(r,o,a){switch(e.calls++,o){case n.TRIANGLES:e.triangles+=a*(r/3);break;case n.LINES:e.lines+=a*(r/2);break;case n.LINE_STRIP:e.lines+=a*(r-1);break;case n.LINE_LOOP:e.lines+=a*r;break;case n.POINTS:e.points+=a*r;break;default:console.error("THREE.WebGLInfo: Unknown draw mode:",o);break}}function s(){e.calls=0,e.triangles=0,e.points=0,e.lines=0}return{memory:t,render:e,programs:null,autoReset:!0,reset:s,update:i}}function t0(n,t,e){let i=new WeakMap,s=new oe;function r(o,a,l){let h=o.morphTargetInfluences,f=a.morphAttributes.position||a.morphAttributes.normal||a.morphAttributes.color,d=f!==void 0?f.length:0,c=i.get(a);if(c===void 0||c.count!==d){let H=function(){E.dispose(),i.delete(a),a.removeEventListener("dispose",H)};c!==void 0&&c.texture.dispose();let u=a.morphAttributes.position!==void 0,g=a.morphAttributes.normal!==void 0,y=a.morphAttributes.color!==void 0,p=a.morphAttributes.position||[],m=a.morphAttributes.normal||[],w=a.morphAttributes.color||[],_=0;u===!0&&(_=1),g===!0&&(_=2),y===!0&&(_=3);let A=a.attributes.position.count*_,I=1;A>t.maxTextureSize&&(I=Math.ceil(A/t.maxTextureSize),A=t.maxTextureSize);let C=new Float32Array(A*I*4*d),E=new no(C,A,I,d);E.type=xn,E.needsUpdate=!0;let T=_*4;for(let x=0;x<d;x++){let S=p[x],O=m[x],W=w[x],q=A*I*4*x;for(let et=0;et<S.count;et++){let F=et*T;u===!0&&(s.fromBufferAttribute(S,et),C[q+F+0]=s.x,C[q+F+1]=s.y,C[q+F+2]=s.z,C[q+F+3]=0),g===!0&&(s.fromBufferAttribute(O,et),C[q+F+4]=s.x,C[q+F+5]=s.y,C[q+F+6]=s.z,C[q+F+7]=0),y===!0&&(s.fromBufferAttribute(W,et),C[q+F+8]=s.x,C[q+F+9]=s.y,C[q+F+10]=s.z,C[q+F+11]=W.itemSize===4?s.w:1)}}c={count:d,texture:E,size:new yt(A,I)},i.set(a,c),a.addEventListener("dispose",H)}if(o.isInstancedMesh===!0&&o.morphTexture!==null)l.getUniforms().setValue(n,"morphTexture",o.morphTexture,e);else{let u=0;for(let y=0;y<h.length;y++)u+=h[y];let g=a.morphTargetsRelative?1:1-u;l.getUniforms().setValue(n,"morphTargetBaseInfluence",g),l.getUniforms().setValue(n,"morphTargetInfluences",h)}l.getUniforms().setValue(n,"morphTargetsTexture",c.texture,e),l.getUniforms().setValue(n,"morphTargetsTextureSize",c.size)}return{update:r}}function e0(n,t,e,i){let s=new WeakMap;function r(l){let h=i.render.frame,f=l.geometry,d=t.get(l,f);if(s.get(d)!==h&&(t.update(d),s.set(d,h)),l.isInstancedMesh&&(l.hasEventListener("dispose",a)===!1&&l.addEventListener("dispose",a),s.get(l)!==h&&(e.update(l.instanceMatrix,n.ARRAY_BUFFER),l.instanceColor!==null&&e.update(l.instanceColor,n.ARRAY_BUFFER),s.set(l,h))),l.isSkinnedMesh){let c=l.skeleton;s.get(c)!==h&&(c.update(),s.set(c,h))}return d}function o(){s=new WeakMap}function a(l){let h=l.target;h.removeEventListener("dispose",a),e.remove(h.instanceMatrix),h.instanceColor!==null&&e.remove(h.instanceColor)}return{update:r,dispose:o}}var ho=class extends Ce{constructor(t,e,i,s,r,o,a,l,h,f=ss){if(f!==ss&&f!==ds)throw new Error("DepthTexture format must be either THREE.DepthFormat or THREE.DepthStencilFormat");i===void 0&&f===ss&&(i=vi),i===void 0&&f===ds&&(i=us),super(null,s,r,o,a,l,f,i,h),this.isDepthTexture=!0,this.image={width:t,height:e},this.magFilter=a!==void 0?a:be,this.minFilter=l!==void 0?l:be,this.flipY=!1,this.generateMipmaps=!1,this.compareFunction=null}copy(t){return super.copy(t),this.compareFunction=t.compareFunction,this}toJSON(t){let e=super.toJSON(t);return this.compareFunction!==null&&(e.compareFunction=this.compareFunction),e}},Eu=new Ce,Bh=new ho(1,1),Tu=new no,Cu=new Pl,Ru=new ao,zh=[],kh=[],Hh=new Float32Array(16),Vh=new Float32Array(9),Gh=new Float32Array(4);function vs(n,t,e){let i=n[0];if(i<=0||i>0)return n;let s=t*e,r=zh[s];if(r===void 0&&(r=new Float32Array(s),zh[s]=r),t!==0){i.toArray(r,0);for(let o=1,a=0;o!==t;++o)a+=e,n[o].toArray(r,a)}return r}function pe(n,t){if(n.length!==t.length)return!1;for(let e=0,i=n.length;e<i;e++)if(n[e]!==t[e])return!1;return!0}function me(n,t){for(let e=0,i=t.length;e<i;e++)n[e]=t[e]}function Eo(n,t){let e=kh[t];e===void 0&&(e=new Int32Array(t),kh[t]=e);for(let i=0;i!==t;++i)e[i]=n.allocateTextureUnit();return e}function n0(n,t){let e=this.cache;e[0]!==t&&(n.uniform1f(this.addr,t),e[0]=t)}function i0(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y)&&(n.uniform2f(this.addr,t.x,t.y),e[0]=t.x,e[1]=t.y);else{if(pe(e,t))return;n.uniform2fv(this.addr,t),me(e,t)}}function s0(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y||e[2]!==t.z)&&(n.uniform3f(this.addr,t.x,t.y,t.z),e[0]=t.x,e[1]=t.y,e[2]=t.z);else if(t.r!==void 0)(e[0]!==t.r||e[1]!==t.g||e[2]!==t.b)&&(n.uniform3f(this.addr,t.r,t.g,t.b),e[0]=t.r,e[1]=t.g,e[2]=t.b);else{if(pe(e,t))return;n.uniform3fv(this.addr,t),me(e,t)}}function r0(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y||e[2]!==t.z||e[3]!==t.w)&&(n.uniform4f(this.addr,t.x,t.y,t.z,t.w),e[0]=t.x,e[1]=t.y,e[2]=t.z,e[3]=t.w);else{if(pe(e,t))return;n.uniform4fv(this.addr,t),me(e,t)}}function o0(n,t){let e=this.cache,i=t.elements;if(i===void 0){if(pe(e,t))return;n.uniformMatrix2fv(this.addr,!1,t),me(e,t)}else{if(pe(e,i))return;Gh.set(i),n.uniformMatrix2fv(this.addr,!1,Gh),me(e,i)}}function a0(n,t){let e=this.cache,i=t.elements;if(i===void 0){if(pe(e,t))return;n.uniformMatrix3fv(this.addr,!1,t),me(e,t)}else{if(pe(e,i))return;Vh.set(i),n.uniformMatrix3fv(this.addr,!1,Vh),me(e,i)}}function l0(n,t){let e=this.cache,i=t.elements;if(i===void 0){if(pe(e,t))return;n.uniformMatrix4fv(this.addr,!1,t),me(e,t)}else{if(pe(e,i))return;Hh.set(i),n.uniformMatrix4fv(this.addr,!1,Hh),me(e,i)}}function c0(n,t){let e=this.cache;e[0]!==t&&(n.uniform1i(this.addr,t),e[0]=t)}function h0(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y)&&(n.uniform2i(this.addr,t.x,t.y),e[0]=t.x,e[1]=t.y);else{if(pe(e,t))return;n.uniform2iv(this.addr,t),me(e,t)}}function u0(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y||e[2]!==t.z)&&(n.uniform3i(this.addr,t.x,t.y,t.z),e[0]=t.x,e[1]=t.y,e[2]=t.z);else{if(pe(e,t))return;n.uniform3iv(this.addr,t),me(e,t)}}function d0(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y||e[2]!==t.z||e[3]!==t.w)&&(n.uniform4i(this.addr,t.x,t.y,t.z,t.w),e[0]=t.x,e[1]=t.y,e[2]=t.z,e[3]=t.w);else{if(pe(e,t))return;n.uniform4iv(this.addr,t),me(e,t)}}function f0(n,t){let e=this.cache;e[0]!==t&&(n.uniform1ui(this.addr,t),e[0]=t)}function p0(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y)&&(n.uniform2ui(this.addr,t.x,t.y),e[0]=t.x,e[1]=t.y);else{if(pe(e,t))return;n.uniform2uiv(this.addr,t),me(e,t)}}function m0(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y||e[2]!==t.z)&&(n.uniform3ui(this.addr,t.x,t.y,t.z),e[0]=t.x,e[1]=t.y,e[2]=t.z);else{if(pe(e,t))return;n.uniform3uiv(this.addr,t),me(e,t)}}function g0(n,t){let e=this.cache;if(t.x!==void 0)(e[0]!==t.x||e[1]!==t.y||e[2]!==t.z||e[3]!==t.w)&&(n.uniform4ui(this.addr,t.x,t.y,t.z,t.w),e[0]=t.x,e[1]=t.y,e[2]=t.z,e[3]=t.w);else{if(pe(e,t))return;n.uniform4uiv(this.addr,t),me(e,t)}}function x0(n,t,e){let i=this.cache,s=e.allocateTextureUnit();i[0]!==s&&(n.uniform1i(this.addr,s),i[0]=s);let r;this.type===n.SAMPLER_2D_SHADOW?(Bh.compareFunction=Mu,r=Bh):r=Eu,e.setTexture2D(t||r,s)}function v0(n,t,e){let i=this.cache,s=e.allocateTextureUnit();i[0]!==s&&(n.uniform1i(this.addr,s),i[0]=s),e.setTexture3D(t||Cu,s)}function _0(n,t,e){let i=this.cache,s=e.allocateTextureUnit();i[0]!==s&&(n.uniform1i(this.addr,s),i[0]=s),e.setTextureCube(t||Ru,s)}function y0(n,t,e){let i=this.cache,s=e.allocateTextureUnit();i[0]!==s&&(n.uniform1i(this.addr,s),i[0]=s),e.setTexture2DArray(t||Tu,s)}function M0(n){switch(n){case 5126:return n0;case 35664:return i0;case 35665:return s0;case 35666:return r0;case 35674:return o0;case 35675:return a0;case 35676:return l0;case 5124:case 35670:return c0;case 35667:case 35671:return h0;case 35668:case 35672:return u0;case 35669:case 35673:return d0;case 5125:return f0;case 36294:return p0;case 36295:return m0;case 36296:return g0;case 35678:case 36198:case 36298:case 36306:case 35682:return x0;case 35679:case 36299:case 36307:return v0;case 35680:case 36300:case 36308:case 36293:return _0;case 36289:case 36303:case 36311:case 36292:return y0}}function S0(n,t){n.uniform1fv(this.addr,t)}function b0(n,t){let e=vs(t,this.size,2);n.uniform2fv(this.addr,e)}function w0(n,t){let e=vs(t,this.size,3);n.uniform3fv(this.addr,e)}function A0(n,t){let e=vs(t,this.size,4);n.uniform4fv(this.addr,e)}function E0(n,t){let e=vs(t,this.size,4);n.uniformMatrix2fv(this.addr,!1,e)}function T0(n,t){let e=vs(t,this.size,9);n.uniformMatrix3fv(this.addr,!1,e)}function C0(n,t){let e=vs(t,this.size,16);n.uniformMatrix4fv(this.addr,!1,e)}function R0(n,t){n.uniform1iv(this.addr,t)}function P0(n,t){n.uniform2iv(this.addr,t)}function I0(n,t){n.uniform3iv(this.addr,t)}function L0(n,t){n.uniform4iv(this.addr,t)}function D0(n,t){n.uniform1uiv(this.addr,t)}function U0(n,t){n.uniform2uiv(this.addr,t)}function N0(n,t){n.uniform3uiv(this.addr,t)}function O0(n,t){n.uniform4uiv(this.addr,t)}function F0(n,t,e){let i=this.cache,s=t.length,r=Eo(e,s);pe(i,r)||(n.uniform1iv(this.addr,r),me(i,r));for(let o=0;o!==s;++o)e.setTexture2D(t[o]||Eu,r[o])}function B0(n,t,e){let i=this.cache,s=t.length,r=Eo(e,s);pe(i,r)||(n.uniform1iv(this.addr,r),me(i,r));for(let o=0;o!==s;++o)e.setTexture3D(t[o]||Cu,r[o])}function z0(n,t,e){let i=this.cache,s=t.length,r=Eo(e,s);pe(i,r)||(n.uniform1iv(this.addr,r),me(i,r));for(let o=0;o!==s;++o)e.setTextureCube(t[o]||Ru,r[o])}function k0(n,t,e){let i=this.cache,s=t.length,r=Eo(e,s);pe(i,r)||(n.uniform1iv(this.addr,r),me(i,r));for(let o=0;o!==s;++o)e.setTexture2DArray(t[o]||Tu,r[o])}function H0(n){switch(n){case 5126:return S0;case 35664:return b0;case 35665:return w0;case 35666:return A0;case 35674:return E0;case 35675:return T0;case 35676:return C0;case 5124:case 35670:return R0;case 35667:case 35671:return P0;case 35668:case 35672:return I0;case 35669:case 35673:return L0;case 5125:return D0;case 36294:return U0;case 36295:return N0;case 36296:return O0;case 35678:case 36198:case 36298:case 36306:case 35682:return F0;case 35679:case 36299:case 36307:return B0;case 35680:case 36300:case 36308:case 36293:return z0;case 36289:case 36303:case 36311:case 36292:return k0}}var Dl=class{constructor(t,e,i){this.id=t,this.addr=i,this.cache=[],this.type=e.type,this.setValue=M0(e.type)}},Ul=class{constructor(t,e,i){this.id=t,this.addr=i,this.cache=[],this.type=e.type,this.size=e.size,this.setValue=H0(e.type)}},Nl=class{constructor(t){this.id=t,this.seq=[],this.map={}}setValue(t,e,i){let s=this.seq;for(let r=0,o=s.length;r!==o;++r){let a=s[r];a.setValue(t,e[a.id],i)}}},ka=/(\w+)(\])?(\[|\.)?/g;function Wh(n,t){n.seq.push(t),n.map[t.id]=t}function V0(n,t,e){let i=n.name,s=i.length;for(ka.lastIndex=0;;){let r=ka.exec(i),o=ka.lastIndex,a=r[1],l=r[2]==="]",h=r[3];if(l&&(a=a|0),h===void 0||h==="["&&o+2===s){Wh(e,h===void 0?new Dl(a,n,t):new Ul(a,n,t));break}else{let d=e.map[a];d===void 0&&(d=new Nl(a),Wh(e,d)),e=d}}}var os=class{constructor(t,e){this.seq=[],this.map={};let i=t.getProgramParameter(e,t.ACTIVE_UNIFORMS);for(let s=0;s<i;++s){let r=t.getActiveUniform(e,s),o=t.getUniformLocation(e,r.name);V0(r,o,this)}}setValue(t,e,i,s){let r=this.map[e];r!==void 0&&r.setValue(t,i,s)}setOptional(t,e,i){let s=e[i];s!==void 0&&this.setValue(t,i,s)}static upload(t,e,i,s){for(let r=0,o=e.length;r!==o;++r){let a=e[r],l=i[a.id];l.needsUpdate!==!1&&a.setValue(t,l.value,s)}}static seqWithValue(t,e){let i=[];for(let s=0,r=t.length;s!==r;++s){let o=t[s];o.id in e&&i.push(o)}return i}};function Xh(n,t,e){let i=n.createShader(t);return n.shaderSource(i,e),n.compileShader(i),i}var G0=37297,W0=0;function X0(n,t){let e=n.split(`
`),i=[],s=Math.max(t-6,0),r=Math.min(t+6,e.length);for(let o=s;o<r;o++){let a=o+1;i.push(`${a===t?">":" "} ${a}: ${e[o]}`)}return i.join(`
`)}function q0(n){let t=Jt.getPrimaries(Jt.workingColorSpace),e=Jt.getPrimaries(n),i;switch(t===e?i="":t===Qr&&e===Jr?i="LinearDisplayP3ToLinearSRGB":t===Jr&&e===Qr&&(i="LinearSRGBToLinearDisplayP3"),n){case Kn:case Ao:return[i,"LinearTransferOETF"];case mn:case dc:return[i,"sRGBTransferOETF"];default:return console.warn("THREE.WebGLProgram: Unsupported color space:",n),[i,"LinearTransferOETF"]}}function qh(n,t,e){let i=n.getShaderParameter(t,n.COMPILE_STATUS),s=n.getShaderInfoLog(t).trim();if(i&&s==="")return"";let r=/ERROR: 0:(\d+)/.exec(s);if(r){let o=parseInt(r[1]);return e.toUpperCase()+`

`+s+`

`+X0(n.getShaderSource(t),o)}else return s}function Y0(n,t){let e=q0(t);return`vec4 ${n}( vec4 value ) { return ${e[0]}( ${e[1]}( value ) ); }`}function Z0(n,t){let e;switch(t){case sf:e="Linear";break;case rf:e="Reinhard";break;case of:e="Cineon";break;case af:e="ACESFilmic";break;case cf:e="AgX";break;case hf:e="Neutral";break;case lf:e="Custom";break;default:console.warn("THREE.WebGLProgram: Unsupported toneMapping:",t),e="Linear"}return"vec3 "+n+"( vec3 color ) { return "+e+"ToneMapping( color ); }"}var kr=new U;function j0(){Jt.getLuminanceCoefficients(kr);let n=kr.x.toFixed(4),t=kr.y.toFixed(4),e=kr.z.toFixed(4);return["float luminance( const in vec3 rgb ) {",`	const vec3 weights = vec3( ${n}, ${t}, ${e} );`,"	return dot( weights, rgb );","}"].join(`
`)}function K0(n){return[n.extensionClipCullDistance?"#extension GL_ANGLE_clip_cull_distance : require":"",n.extensionMultiDraw?"#extension GL_ANGLE_multi_draw : require":""].filter(Zs).join(`
`)}function J0(n){let t=[];for(let e in n){let i=n[e];i!==!1&&t.push("#define "+e+" "+i)}return t.join(`
`)}function Q0(n,t){let e={},i=n.getProgramParameter(t,n.ACTIVE_ATTRIBUTES);for(let s=0;s<i;s++){let r=n.getActiveAttrib(t,s),o=r.name,a=1;r.type===n.FLOAT_MAT2&&(a=2),r.type===n.FLOAT_MAT3&&(a=3),r.type===n.FLOAT_MAT4&&(a=4),e[o]={type:r.type,location:n.getAttribLocation(t,o),locationSize:a}}return e}function Zs(n){return n!==""}function Yh(n,t){let e=t.numSpotLightShadows+t.numSpotLightMaps-t.numSpotLightShadowsWithMaps;return n.replace(/NUM_DIR_LIGHTS/g,t.numDirLights).replace(/NUM_SPOT_LIGHTS/g,t.numSpotLights).replace(/NUM_SPOT_LIGHT_MAPS/g,t.numSpotLightMaps).replace(/NUM_SPOT_LIGHT_COORDS/g,e).replace(/NUM_RECT_AREA_LIGHTS/g,t.numRectAreaLights).replace(/NUM_POINT_LIGHTS/g,t.numPointLights).replace(/NUM_HEMI_LIGHTS/g,t.numHemiLights).replace(/NUM_DIR_LIGHT_SHADOWS/g,t.numDirLightShadows).replace(/NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS/g,t.numSpotLightShadowsWithMaps).replace(/NUM_SPOT_LIGHT_SHADOWS/g,t.numSpotLightShadows).replace(/NUM_POINT_LIGHT_SHADOWS/g,t.numPointLightShadows)}function Zh(n,t){return n.replace(/NUM_CLIPPING_PLANES/g,t.numClippingPlanes).replace(/UNION_CLIPPING_PLANES/g,t.numClippingPlanes-t.numClipIntersection)}var $0=/^[ \t]*#include +<([\w\d./]+)>/gm;function Ol(n){return n.replace($0,ex)}var tx=new Map;function ex(n,t){let e=Bt[t];if(e===void 0){let i=tx.get(t);if(i!==void 0)e=Bt[i],console.warn('THREE.WebGLRenderer: Shader chunk "%s" has been deprecated. Use "%s" instead.',t,i);else throw new Error("Can not resolve #include <"+t+">")}return Ol(e)}var nx=/#pragma unroll_loop_start\s+for\s*\(\s*int\s+i\s*=\s*(\d+)\s*;\s*i\s*<\s*(\d+)\s*;\s*i\s*\+\+\s*\)\s*{([\s\S]+?)}\s+#pragma unroll_loop_end/g;function jh(n){return n.replace(nx,ix)}function ix(n,t,e,i){let s="";for(let r=parseInt(t);r<parseInt(e);r++)s+=i.replace(/\[\s*i\s*\]/g,"[ "+r+" ]").replace(/UNROLLED_LOOP_INDEX/g,r);return s}function Kh(n){let t=`precision ${n.precision} float;
	precision ${n.precision} int;
	precision ${n.precision} sampler2D;
	precision ${n.precision} samplerCube;
	precision ${n.precision} sampler3D;
	precision ${n.precision} sampler2DArray;
	precision ${n.precision} sampler2DShadow;
	precision ${n.precision} samplerCubeShadow;
	precision ${n.precision} sampler2DArrayShadow;
	precision ${n.precision} isampler2D;
	precision ${n.precision} isampler3D;
	precision ${n.precision} isamplerCube;
	precision ${n.precision} isampler2DArray;
	precision ${n.precision} usampler2D;
	precision ${n.precision} usampler3D;
	precision ${n.precision} usamplerCube;
	precision ${n.precision} usampler2DArray;
	`;return n.precision==="highp"?t+=`
#define HIGH_PRECISION`:n.precision==="mediump"?t+=`
#define MEDIUM_PRECISION`:n.precision==="lowp"&&(t+=`
#define LOW_PRECISION`),t}function sx(n){let t="SHADOWMAP_TYPE_BASIC";return n.shadowMapType===lu?t="SHADOWMAP_TYPE_PCF":n.shadowMapType===Od?t="SHADOWMAP_TYPE_PCF_SOFT":n.shadowMapType===Cn&&(t="SHADOWMAP_TYPE_VSM"),t}function rx(n){let t="ENVMAP_TYPE_CUBE";if(n.envMap)switch(n.envMapMode){case cs:case hs:t="ENVMAP_TYPE_CUBE";break;case wo:t="ENVMAP_TYPE_CUBE_UV";break}return t}function ox(n){let t="ENVMAP_MODE_REFLECTION";return n.envMap&&n.envMapMode===hs&&(t="ENVMAP_MODE_REFRACTION"),t}function ax(n){let t="ENVMAP_BLENDING_NONE";if(n.envMap)switch(n.combine){case cu:t="ENVMAP_BLENDING_MULTIPLY";break;case ef:t="ENVMAP_BLENDING_MIX";break;case nf:t="ENVMAP_BLENDING_ADD";break}return t}function lx(n){let t=n.envMapCubeUVHeight;if(t===null)return null;let e=Math.log2(t)-2,i=1/t;return{texelWidth:1/(3*Math.max(Math.pow(2,e),112)),texelHeight:i,maxMip:e}}function cx(n,t,e,i){let s=n.getContext(),r=e.defines,o=e.vertexShader,a=e.fragmentShader,l=sx(e),h=rx(e),f=ox(e),d=ax(e),c=lx(e),u=K0(e),g=J0(r),y=s.createProgram(),p,m,w=e.glslVersion?"#version "+e.glslVersion+`
`:"";e.isRawShaderMaterial?(p=["#define SHADER_TYPE "+e.shaderType,"#define SHADER_NAME "+e.shaderName,g].filter(Zs).join(`
`),p.length>0&&(p+=`
`),m=["#define SHADER_TYPE "+e.shaderType,"#define SHADER_NAME "+e.shaderName,g].filter(Zs).join(`
`),m.length>0&&(m+=`
`)):(p=[Kh(e),"#define SHADER_TYPE "+e.shaderType,"#define SHADER_NAME "+e.shaderName,g,e.extensionClipCullDistance?"#define USE_CLIP_DISTANCE":"",e.batching?"#define USE_BATCHING":"",e.batchingColor?"#define USE_BATCHING_COLOR":"",e.instancing?"#define USE_INSTANCING":"",e.instancingColor?"#define USE_INSTANCING_COLOR":"",e.instancingMorph?"#define USE_INSTANCING_MORPH":"",e.useFog&&e.fog?"#define USE_FOG":"",e.useFog&&e.fogExp2?"#define FOG_EXP2":"",e.map?"#define USE_MAP":"",e.envMap?"#define USE_ENVMAP":"",e.envMap?"#define "+f:"",e.lightMap?"#define USE_LIGHTMAP":"",e.aoMap?"#define USE_AOMAP":"",e.bumpMap?"#define USE_BUMPMAP":"",e.normalMap?"#define USE_NORMALMAP":"",e.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",e.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",e.displacementMap?"#define USE_DISPLACEMENTMAP":"",e.emissiveMap?"#define USE_EMISSIVEMAP":"",e.anisotropy?"#define USE_ANISOTROPY":"",e.anisotropyMap?"#define USE_ANISOTROPYMAP":"",e.clearcoatMap?"#define USE_CLEARCOATMAP":"",e.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",e.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",e.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",e.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",e.specularMap?"#define USE_SPECULARMAP":"",e.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",e.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",e.roughnessMap?"#define USE_ROUGHNESSMAP":"",e.metalnessMap?"#define USE_METALNESSMAP":"",e.alphaMap?"#define USE_ALPHAMAP":"",e.alphaHash?"#define USE_ALPHAHASH":"",e.transmission?"#define USE_TRANSMISSION":"",e.transmissionMap?"#define USE_TRANSMISSIONMAP":"",e.thicknessMap?"#define USE_THICKNESSMAP":"",e.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",e.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",e.mapUv?"#define MAP_UV "+e.mapUv:"",e.alphaMapUv?"#define ALPHAMAP_UV "+e.alphaMapUv:"",e.lightMapUv?"#define LIGHTMAP_UV "+e.lightMapUv:"",e.aoMapUv?"#define AOMAP_UV "+e.aoMapUv:"",e.emissiveMapUv?"#define EMISSIVEMAP_UV "+e.emissiveMapUv:"",e.bumpMapUv?"#define BUMPMAP_UV "+e.bumpMapUv:"",e.normalMapUv?"#define NORMALMAP_UV "+e.normalMapUv:"",e.displacementMapUv?"#define DISPLACEMENTMAP_UV "+e.displacementMapUv:"",e.metalnessMapUv?"#define METALNESSMAP_UV "+e.metalnessMapUv:"",e.roughnessMapUv?"#define ROUGHNESSMAP_UV "+e.roughnessMapUv:"",e.anisotropyMapUv?"#define ANISOTROPYMAP_UV "+e.anisotropyMapUv:"",e.clearcoatMapUv?"#define CLEARCOATMAP_UV "+e.clearcoatMapUv:"",e.clearcoatNormalMapUv?"#define CLEARCOAT_NORMALMAP_UV "+e.clearcoatNormalMapUv:"",e.clearcoatRoughnessMapUv?"#define CLEARCOAT_ROUGHNESSMAP_UV "+e.clearcoatRoughnessMapUv:"",e.iridescenceMapUv?"#define IRIDESCENCEMAP_UV "+e.iridescenceMapUv:"",e.iridescenceThicknessMapUv?"#define IRIDESCENCE_THICKNESSMAP_UV "+e.iridescenceThicknessMapUv:"",e.sheenColorMapUv?"#define SHEEN_COLORMAP_UV "+e.sheenColorMapUv:"",e.sheenRoughnessMapUv?"#define SHEEN_ROUGHNESSMAP_UV "+e.sheenRoughnessMapUv:"",e.specularMapUv?"#define SPECULARMAP_UV "+e.specularMapUv:"",e.specularColorMapUv?"#define SPECULAR_COLORMAP_UV "+e.specularColorMapUv:"",e.specularIntensityMapUv?"#define SPECULAR_INTENSITYMAP_UV "+e.specularIntensityMapUv:"",e.transmissionMapUv?"#define TRANSMISSIONMAP_UV "+e.transmissionMapUv:"",e.thicknessMapUv?"#define THICKNESSMAP_UV "+e.thicknessMapUv:"",e.vertexTangents&&e.flatShading===!1?"#define USE_TANGENT":"",e.vertexColors?"#define USE_COLOR":"",e.vertexAlphas?"#define USE_COLOR_ALPHA":"",e.vertexUv1s?"#define USE_UV1":"",e.vertexUv2s?"#define USE_UV2":"",e.vertexUv3s?"#define USE_UV3":"",e.pointsUvs?"#define USE_POINTS_UV":"",e.flatShading?"#define FLAT_SHADED":"",e.skinning?"#define USE_SKINNING":"",e.morphTargets?"#define USE_MORPHTARGETS":"",e.morphNormals&&e.flatShading===!1?"#define USE_MORPHNORMALS":"",e.morphColors?"#define USE_MORPHCOLORS":"",e.morphTargetsCount>0?"#define MORPHTARGETS_TEXTURE_STRIDE "+e.morphTextureStride:"",e.morphTargetsCount>0?"#define MORPHTARGETS_COUNT "+e.morphTargetsCount:"",e.doubleSided?"#define DOUBLE_SIDED":"",e.flipSided?"#define FLIP_SIDED":"",e.shadowMapEnabled?"#define USE_SHADOWMAP":"",e.shadowMapEnabled?"#define "+l:"",e.sizeAttenuation?"#define USE_SIZEATTENUATION":"",e.numLightProbes>0?"#define USE_LIGHT_PROBES":"",e.logarithmicDepthBuffer?"#define USE_LOGDEPTHBUF":"",e.reverseDepthBuffer?"#define USE_REVERSEDEPTHBUF":"","uniform mat4 modelMatrix;","uniform mat4 modelViewMatrix;","uniform mat4 projectionMatrix;","uniform mat4 viewMatrix;","uniform mat3 normalMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;","#ifdef USE_INSTANCING","	attribute mat4 instanceMatrix;","#endif","#ifdef USE_INSTANCING_COLOR","	attribute vec3 instanceColor;","#endif","#ifdef USE_INSTANCING_MORPH","	uniform sampler2D morphTexture;","#endif","attribute vec3 position;","attribute vec3 normal;","attribute vec2 uv;","#ifdef USE_UV1","	attribute vec2 uv1;","#endif","#ifdef USE_UV2","	attribute vec2 uv2;","#endif","#ifdef USE_UV3","	attribute vec2 uv3;","#endif","#ifdef USE_TANGENT","	attribute vec4 tangent;","#endif","#if defined( USE_COLOR_ALPHA )","	attribute vec4 color;","#elif defined( USE_COLOR )","	attribute vec3 color;","#endif","#ifdef USE_SKINNING","	attribute vec4 skinIndex;","	attribute vec4 skinWeight;","#endif",`
`].filter(Zs).join(`
`),m=[Kh(e),"#define SHADER_TYPE "+e.shaderType,"#define SHADER_NAME "+e.shaderName,g,e.useFog&&e.fog?"#define USE_FOG":"",e.useFog&&e.fogExp2?"#define FOG_EXP2":"",e.alphaToCoverage?"#define ALPHA_TO_COVERAGE":"",e.map?"#define USE_MAP":"",e.matcap?"#define USE_MATCAP":"",e.envMap?"#define USE_ENVMAP":"",e.envMap?"#define "+h:"",e.envMap?"#define "+f:"",e.envMap?"#define "+d:"",c?"#define CUBEUV_TEXEL_WIDTH "+c.texelWidth:"",c?"#define CUBEUV_TEXEL_HEIGHT "+c.texelHeight:"",c?"#define CUBEUV_MAX_MIP "+c.maxMip+".0":"",e.lightMap?"#define USE_LIGHTMAP":"",e.aoMap?"#define USE_AOMAP":"",e.bumpMap?"#define USE_BUMPMAP":"",e.normalMap?"#define USE_NORMALMAP":"",e.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",e.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",e.emissiveMap?"#define USE_EMISSIVEMAP":"",e.anisotropy?"#define USE_ANISOTROPY":"",e.anisotropyMap?"#define USE_ANISOTROPYMAP":"",e.clearcoat?"#define USE_CLEARCOAT":"",e.clearcoatMap?"#define USE_CLEARCOATMAP":"",e.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",e.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",e.dispersion?"#define USE_DISPERSION":"",e.iridescence?"#define USE_IRIDESCENCE":"",e.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",e.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",e.specularMap?"#define USE_SPECULARMAP":"",e.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",e.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",e.roughnessMap?"#define USE_ROUGHNESSMAP":"",e.metalnessMap?"#define USE_METALNESSMAP":"",e.alphaMap?"#define USE_ALPHAMAP":"",e.alphaTest?"#define USE_ALPHATEST":"",e.alphaHash?"#define USE_ALPHAHASH":"",e.sheen?"#define USE_SHEEN":"",e.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",e.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",e.transmission?"#define USE_TRANSMISSION":"",e.transmissionMap?"#define USE_TRANSMISSIONMAP":"",e.thicknessMap?"#define USE_THICKNESSMAP":"",e.vertexTangents&&e.flatShading===!1?"#define USE_TANGENT":"",e.vertexColors||e.instancingColor||e.batchingColor?"#define USE_COLOR":"",e.vertexAlphas?"#define USE_COLOR_ALPHA":"",e.vertexUv1s?"#define USE_UV1":"",e.vertexUv2s?"#define USE_UV2":"",e.vertexUv3s?"#define USE_UV3":"",e.pointsUvs?"#define USE_POINTS_UV":"",e.gradientMap?"#define USE_GRADIENTMAP":"",e.flatShading?"#define FLAT_SHADED":"",e.doubleSided?"#define DOUBLE_SIDED":"",e.flipSided?"#define FLIP_SIDED":"",e.shadowMapEnabled?"#define USE_SHADOWMAP":"",e.shadowMapEnabled?"#define "+l:"",e.premultipliedAlpha?"#define PREMULTIPLIED_ALPHA":"",e.numLightProbes>0?"#define USE_LIGHT_PROBES":"",e.decodeVideoTexture?"#define DECODE_VIDEO_TEXTURE":"",e.logarithmicDepthBuffer?"#define USE_LOGDEPTHBUF":"",e.reverseDepthBuffer?"#define USE_REVERSEDEPTHBUF":"","uniform mat4 viewMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;",e.toneMapping!==Zn?"#define TONE_MAPPING":"",e.toneMapping!==Zn?Bt.tonemapping_pars_fragment:"",e.toneMapping!==Zn?Z0("toneMapping",e.toneMapping):"",e.dithering?"#define DITHERING":"",e.opaque?"#define OPAQUE":"",Bt.colorspace_pars_fragment,Y0("linearToOutputTexel",e.outputColorSpace),j0(),e.useDepthPacking?"#define DEPTH_PACKING "+e.depthPacking:"",`
`].filter(Zs).join(`
`)),o=Ol(o),o=Yh(o,e),o=Zh(o,e),a=Ol(a),a=Yh(a,e),a=Zh(a,e),o=jh(o),a=jh(a),e.isRawShaderMaterial!==!0&&(w=`#version 300 es
`,p=[u,"#define attribute in","#define varying out","#define texture2D texture"].join(`
`)+`
`+p,m=["#define varying in",e.glslVersion===fh?"":"layout(location = 0) out highp vec4 pc_fragColor;",e.glslVersion===fh?"":"#define gl_FragColor pc_fragColor","#define gl_FragDepthEXT gl_FragDepth","#define texture2D texture","#define textureCube texture","#define texture2DProj textureProj","#define texture2DLodEXT textureLod","#define texture2DProjLodEXT textureProjLod","#define textureCubeLodEXT textureLod","#define texture2DGradEXT textureGrad","#define texture2DProjGradEXT textureProjGrad","#define textureCubeGradEXT textureGrad"].join(`
`)+`
`+m);let _=w+p+o,A=w+m+a,I=Xh(s,s.VERTEX_SHADER,_),C=Xh(s,s.FRAGMENT_SHADER,A);s.attachShader(y,I),s.attachShader(y,C),e.index0AttributeName!==void 0?s.bindAttribLocation(y,0,e.index0AttributeName):e.morphTargets===!0&&s.bindAttribLocation(y,0,"position"),s.linkProgram(y);function E(S){if(n.debug.checkShaderErrors){let O=s.getProgramInfoLog(y).trim(),W=s.getShaderInfoLog(I).trim(),q=s.getShaderInfoLog(C).trim(),et=!0,F=!0;if(s.getProgramParameter(y,s.LINK_STATUS)===!1)if(et=!1,typeof n.debug.onShaderError=="function")n.debug.onShaderError(s,y,I,C);else{let it=qh(s,I,"vertex"),Z=qh(s,C,"fragment");console.error("THREE.WebGLProgram: Shader Error "+s.getError()+" - VALIDATE_STATUS "+s.getProgramParameter(y,s.VALIDATE_STATUS)+`

Material Name: `+S.name+`
Material Type: `+S.type+`

Program Info Log: `+O+`
`+it+`
`+Z)}else O!==""?console.warn("THREE.WebGLProgram: Program Info Log:",O):(W===""||q==="")&&(F=!1);F&&(S.diagnostics={runnable:et,programLog:O,vertexShader:{log:W,prefix:p},fragmentShader:{log:q,prefix:m}})}s.deleteShader(I),s.deleteShader(C),T=new os(s,y),H=Q0(s,y)}let T;this.getUniforms=function(){return T===void 0&&E(this),T};let H;this.getAttributes=function(){return H===void 0&&E(this),H};let x=e.rendererExtensionParallelShaderCompile===!1;return this.isReady=function(){return x===!1&&(x=s.getProgramParameter(y,G0)),x},this.destroy=function(){i.releaseStatesOfProgram(this),s.deleteProgram(y),this.program=void 0},this.type=e.shaderType,this.name=e.shaderName,this.id=W0++,this.cacheKey=t,this.usedTimes=1,this.program=y,this.vertexShader=I,this.fragmentShader=C,this}var hx=0,Fl=class{constructor(){this.shaderCache=new Map,this.materialCache=new Map}update(t){let e=t.vertexShader,i=t.fragmentShader,s=this._getShaderStage(e),r=this._getShaderStage(i),o=this._getShaderCacheForMaterial(t);return o.has(s)===!1&&(o.add(s),s.usedTimes++),o.has(r)===!1&&(o.add(r),r.usedTimes++),this}remove(t){let e=this.materialCache.get(t);for(let i of e)i.usedTimes--,i.usedTimes===0&&this.shaderCache.delete(i.code);return this.materialCache.delete(t),this}getVertexShaderID(t){return this._getShaderStage(t.vertexShader).id}getFragmentShaderID(t){return this._getShaderStage(t.fragmentShader).id}dispose(){this.shaderCache.clear(),this.materialCache.clear()}_getShaderCacheForMaterial(t){let e=this.materialCache,i=e.get(t);return i===void 0&&(i=new Set,e.set(t,i)),i}_getShaderStage(t){let e=this.shaderCache,i=e.get(t);return i===void 0&&(i=new Bl(t),e.set(t,i)),i}},Bl=class{constructor(t){this.id=hx++,this.code=t,this.usedTimes=0}};function ux(n,t,e,i,s,r,o){let a=new tr,l=new Fl,h=new Set,f=[],d=s.logarithmicDepthBuffer,c=s.reverseDepthBuffer,u=s.vertexTextures,g=s.precision,y={MeshDepthMaterial:"depth",MeshDistanceMaterial:"distanceRGBA",MeshNormalMaterial:"normal",MeshBasicMaterial:"basic",MeshLambertMaterial:"lambert",MeshPhongMaterial:"phong",MeshToonMaterial:"toon",MeshStandardMaterial:"physical",MeshPhysicalMaterial:"physical",MeshMatcapMaterial:"matcap",LineBasicMaterial:"basic",LineDashedMaterial:"dashed",PointsMaterial:"points",ShadowMaterial:"shadow",SpriteMaterial:"sprite"};function p(x){return h.add(x),x===0?"uv":`uv${x}`}function m(x,S,O,W,q){let et=W.fog,F=q.geometry,it=x.isMeshStandardMaterial?W.environment:null,Z=(x.isMeshStandardMaterial?e:t).get(x.envMap||it),gt=Z&&Z.mapping===wo?Z.image.height:null,_t=y[x.type];x.precision!==null&&(g=s.getMaxPrecision(x.precision),g!==x.precision&&console.warn("THREE.WebGLProgram.getParameters:",x.precision,"not supported, using",g,"instead."));let St=F.morphAttributes.position||F.morphAttributes.normal||F.morphAttributes.color,Gt=St!==void 0?St.length:0,Wt=0;F.morphAttributes.position!==void 0&&(Wt=1),F.morphAttributes.normal!==void 0&&(Wt=2),F.morphAttributes.color!==void 0&&(Wt=3);let J,st,Mt,xt;if(_t){let Oe=gn[_t];J=Oe.vertexShader,st=Oe.fragmentShader}else J=x.vertexShader,st=x.fragmentShader,l.update(x),Mt=l.getVertexShaderID(x),xt=l.getFragmentShaderID(x);let Nt=n.getRenderTarget(),It=q.isInstancedMesh===!0,kt=q.isBatchedMesh===!0,Zt=!!x.map,Vt=!!x.matcap,P=!!Z,fe=!!x.aoMap,j=!!x.lightMap,lt=!!x.bumpMap,ut=!!x.normalMap,Rt=!!x.displacementMap,B=!!x.emissiveMap,M=!!x.metalnessMap,v=!!x.roughnessMap,R=x.anisotropy>0,k=x.clearcoat>0,K=x.dispersion>0,z=x.iridescence>0,nt=x.sheen>0,Q=x.transmission>0,X=R&&!!x.anisotropyMap,ht=k&&!!x.clearcoatMap,tt=k&&!!x.clearcoatNormalMap,ct=k&&!!x.clearcoatRoughnessMap,At=z&&!!x.iridescenceMap,rt=z&&!!x.iridescenceThicknessMap,ot=nt&&!!x.sheenColorMap,Lt=nt&&!!x.sheenRoughnessMap,wt=!!x.specularMap,Ft=!!x.specularColorMap,L=!!x.specularIntensityMap,ft=Q&&!!x.transmissionMap,Y=Q&&!!x.thicknessMap,$=!!x.gradientMap,pt=!!x.alphaMap,vt=x.alphaTest>0,Xt=!!x.alphaHash,ce=!!x.extensions,Ne=Zn;x.toneMapped&&(Nt===null||Nt.isXRRenderTarget===!0)&&(Ne=n.toneMapping);let qt={shaderID:_t,shaderType:x.type,shaderName:x.name,vertexShader:J,fragmentShader:st,defines:x.defines,customVertexShaderID:Mt,customFragmentShaderID:xt,isRawShaderMaterial:x.isRawShaderMaterial===!0,glslVersion:x.glslVersion,precision:g,batching:kt,batchingColor:kt&&q._colorsTexture!==null,instancing:It,instancingColor:It&&q.instanceColor!==null,instancingMorph:It&&q.morphTexture!==null,supportsVertexTextures:u,outputColorSpace:Nt===null?n.outputColorSpace:Nt.isXRRenderTarget===!0?Nt.texture.colorSpace:Kn,alphaToCoverage:!!x.alphaToCoverage,map:Zt,matcap:Vt,envMap:P,envMapMode:P&&Z.mapping,envMapCubeUVHeight:gt,aoMap:fe,lightMap:j,bumpMap:lt,normalMap:ut,displacementMap:u&&Rt,emissiveMap:B,normalMapObjectSpace:ut&&x.normalMapType===pf,normalMapTangentSpace:ut&&x.normalMapType===yu,metalnessMap:M,roughnessMap:v,anisotropy:R,anisotropyMap:X,clearcoat:k,clearcoatMap:ht,clearcoatNormalMap:tt,clearcoatRoughnessMap:ct,dispersion:K,iridescence:z,iridescenceMap:At,iridescenceThicknessMap:rt,sheen:nt,sheenColorMap:ot,sheenRoughnessMap:Lt,specularMap:wt,specularColorMap:Ft,specularIntensityMap:L,transmission:Q,transmissionMap:ft,thicknessMap:Y,gradientMap:$,opaque:x.transparent===!1&&x.blending===is&&x.alphaToCoverage===!1,alphaMap:pt,alphaTest:vt,alphaHash:Xt,combine:x.combine,mapUv:Zt&&p(x.map.channel),aoMapUv:fe&&p(x.aoMap.channel),lightMapUv:j&&p(x.lightMap.channel),bumpMapUv:lt&&p(x.bumpMap.channel),normalMapUv:ut&&p(x.normalMap.channel),displacementMapUv:Rt&&p(x.displacementMap.channel),emissiveMapUv:B&&p(x.emissiveMap.channel),metalnessMapUv:M&&p(x.metalnessMap.channel),roughnessMapUv:v&&p(x.roughnessMap.channel),anisotropyMapUv:X&&p(x.anisotropyMap.channel),clearcoatMapUv:ht&&p(x.clearcoatMap.channel),clearcoatNormalMapUv:tt&&p(x.clearcoatNormalMap.channel),clearcoatRoughnessMapUv:ct&&p(x.clearcoatRoughnessMap.channel),iridescenceMapUv:At&&p(x.iridescenceMap.channel),iridescenceThicknessMapUv:rt&&p(x.iridescenceThicknessMap.channel),sheenColorMapUv:ot&&p(x.sheenColorMap.channel),sheenRoughnessMapUv:Lt&&p(x.sheenRoughnessMap.channel),specularMapUv:wt&&p(x.specularMap.channel),specularColorMapUv:Ft&&p(x.specularColorMap.channel),specularIntensityMapUv:L&&p(x.specularIntensityMap.channel),transmissionMapUv:ft&&p(x.transmissionMap.channel),thicknessMapUv:Y&&p(x.thicknessMap.channel),alphaMapUv:pt&&p(x.alphaMap.channel),vertexTangents:!!F.attributes.tangent&&(ut||R),vertexColors:x.vertexColors,vertexAlphas:x.vertexColors===!0&&!!F.attributes.color&&F.attributes.color.itemSize===4,pointsUvs:q.isPoints===!0&&!!F.attributes.uv&&(Zt||pt),fog:!!et,useFog:x.fog===!0,fogExp2:!!et&&et.isFogExp2,flatShading:x.flatShading===!0,sizeAttenuation:x.sizeAttenuation===!0,logarithmicDepthBuffer:d,reverseDepthBuffer:c,skinning:q.isSkinnedMesh===!0,morphTargets:F.morphAttributes.position!==void 0,morphNormals:F.morphAttributes.normal!==void 0,morphColors:F.morphAttributes.color!==void 0,morphTargetsCount:Gt,morphTextureStride:Wt,numDirLights:S.directional.length,numPointLights:S.point.length,numSpotLights:S.spot.length,numSpotLightMaps:S.spotLightMap.length,numRectAreaLights:S.rectArea.length,numHemiLights:S.hemi.length,numDirLightShadows:S.directionalShadowMap.length,numPointLightShadows:S.pointShadowMap.length,numSpotLightShadows:S.spotShadowMap.length,numSpotLightShadowsWithMaps:S.numSpotLightShadowsWithMaps,numLightProbes:S.numLightProbes,numClippingPlanes:o.numPlanes,numClipIntersection:o.numIntersection,dithering:x.dithering,shadowMapEnabled:n.shadowMap.enabled&&O.length>0,shadowMapType:n.shadowMap.type,toneMapping:Ne,decodeVideoTexture:Zt&&x.map.isVideoTexture===!0&&Jt.getTransfer(x.map.colorSpace)===ee,premultipliedAlpha:x.premultipliedAlpha,doubleSided:x.side===Rn,flipSided:x.side===Fe,useDepthPacking:x.depthPacking>=0,depthPacking:x.depthPacking||0,index0AttributeName:x.index0AttributeName,extensionClipCullDistance:ce&&x.extensions.clipCullDistance===!0&&i.has("WEBGL_clip_cull_distance"),extensionMultiDraw:(ce&&x.extensions.multiDraw===!0||kt)&&i.has("WEBGL_multi_draw"),rendererExtensionParallelShaderCompile:i.has("KHR_parallel_shader_compile"),customProgramCacheKey:x.customProgramCacheKey()};return qt.vertexUv1s=h.has(1),qt.vertexUv2s=h.has(2),qt.vertexUv3s=h.has(3),h.clear(),qt}function w(x){let S=[];if(x.shaderID?S.push(x.shaderID):(S.push(x.customVertexShaderID),S.push(x.customFragmentShaderID)),x.defines!==void 0)for(let O in x.defines)S.push(O),S.push(x.defines[O]);return x.isRawShaderMaterial===!1&&(_(S,x),A(S,x),S.push(n.outputColorSpace)),S.push(x.customProgramCacheKey),S.join()}function _(x,S){x.push(S.precision),x.push(S.outputColorSpace),x.push(S.envMapMode),x.push(S.envMapCubeUVHeight),x.push(S.mapUv),x.push(S.alphaMapUv),x.push(S.lightMapUv),x.push(S.aoMapUv),x.push(S.bumpMapUv),x.push(S.normalMapUv),x.push(S.displacementMapUv),x.push(S.emissiveMapUv),x.push(S.metalnessMapUv),x.push(S.roughnessMapUv),x.push(S.anisotropyMapUv),x.push(S.clearcoatMapUv),x.push(S.clearcoatNormalMapUv),x.push(S.clearcoatRoughnessMapUv),x.push(S.iridescenceMapUv),x.push(S.iridescenceThicknessMapUv),x.push(S.sheenColorMapUv),x.push(S.sheenRoughnessMapUv),x.push(S.specularMapUv),x.push(S.specularColorMapUv),x.push(S.specularIntensityMapUv),x.push(S.transmissionMapUv),x.push(S.thicknessMapUv),x.push(S.combine),x.push(S.fogExp2),x.push(S.sizeAttenuation),x.push(S.morphTargetsCount),x.push(S.morphAttributeCount),x.push(S.numDirLights),x.push(S.numPointLights),x.push(S.numSpotLights),x.push(S.numSpotLightMaps),x.push(S.numHemiLights),x.push(S.numRectAreaLights),x.push(S.numDirLightShadows),x.push(S.numPointLightShadows),x.push(S.numSpotLightShadows),x.push(S.numSpotLightShadowsWithMaps),x.push(S.numLightProbes),x.push(S.shadowMapType),x.push(S.toneMapping),x.push(S.numClippingPlanes),x.push(S.numClipIntersection),x.push(S.depthPacking)}function A(x,S){a.disableAll(),S.supportsVertexTextures&&a.enable(0),S.instancing&&a.enable(1),S.instancingColor&&a.enable(2),S.instancingMorph&&a.enable(3),S.matcap&&a.enable(4),S.envMap&&a.enable(5),S.normalMapObjectSpace&&a.enable(6),S.normalMapTangentSpace&&a.enable(7),S.clearcoat&&a.enable(8),S.iridescence&&a.enable(9),S.alphaTest&&a.enable(10),S.vertexColors&&a.enable(11),S.vertexAlphas&&a.enable(12),S.vertexUv1s&&a.enable(13),S.vertexUv2s&&a.enable(14),S.vertexUv3s&&a.enable(15),S.vertexTangents&&a.enable(16),S.anisotropy&&a.enable(17),S.alphaHash&&a.enable(18),S.batching&&a.enable(19),S.dispersion&&a.enable(20),S.batchingColor&&a.enable(21),x.push(a.mask),a.disableAll(),S.fog&&a.enable(0),S.useFog&&a.enable(1),S.flatShading&&a.enable(2),S.logarithmicDepthBuffer&&a.enable(3),S.reverseDepthBuffer&&a.enable(4),S.skinning&&a.enable(5),S.morphTargets&&a.enable(6),S.morphNormals&&a.enable(7),S.morphColors&&a.enable(8),S.premultipliedAlpha&&a.enable(9),S.shadowMapEnabled&&a.enable(10),S.doubleSided&&a.enable(11),S.flipSided&&a.enable(12),S.useDepthPacking&&a.enable(13),S.dithering&&a.enable(14),S.transmission&&a.enable(15),S.sheen&&a.enable(16),S.opaque&&a.enable(17),S.pointsUvs&&a.enable(18),S.decodeVideoTexture&&a.enable(19),S.alphaToCoverage&&a.enable(20),x.push(a.mask)}function I(x){let S=y[x.type],O;if(S){let W=gn[S];O=_n.clone(W.uniforms)}else O=x.uniforms;return O}function C(x,S){let O;for(let W=0,q=f.length;W<q;W++){let et=f[W];if(et.cacheKey===S){O=et,++O.usedTimes;break}}return O===void 0&&(O=new cx(n,S,x,r),f.push(O)),O}function E(x){if(--x.usedTimes===0){let S=f.indexOf(x);f[S]=f[f.length-1],f.pop(),x.destroy()}}function T(x){l.remove(x)}function H(){l.dispose()}return{getParameters:m,getProgramCacheKey:w,getUniforms:I,acquireProgram:C,releaseProgram:E,releaseShaderCache:T,programs:f,dispose:H}}function dx(){let n=new WeakMap;function t(o){return n.has(o)}function e(o){let a=n.get(o);return a===void 0&&(a={},n.set(o,a)),a}function i(o){n.delete(o)}function s(o,a,l){n.get(o)[a]=l}function r(){n=new WeakMap}return{has:t,get:e,remove:i,update:s,dispose:r}}function fx(n,t){return n.groupOrder!==t.groupOrder?n.groupOrder-t.groupOrder:n.renderOrder!==t.renderOrder?n.renderOrder-t.renderOrder:n.material.id!==t.material.id?n.material.id-t.material.id:n.z!==t.z?n.z-t.z:n.id-t.id}function Jh(n,t){return n.groupOrder!==t.groupOrder?n.groupOrder-t.groupOrder:n.renderOrder!==t.renderOrder?n.renderOrder-t.renderOrder:n.z!==t.z?t.z-n.z:n.id-t.id}function Qh(){let n=[],t=0,e=[],i=[],s=[];function r(){t=0,e.length=0,i.length=0,s.length=0}function o(d,c,u,g,y,p){let m=n[t];return m===void 0?(m={id:d.id,object:d,geometry:c,material:u,groupOrder:g,renderOrder:d.renderOrder,z:y,group:p},n[t]=m):(m.id=d.id,m.object=d,m.geometry=c,m.material=u,m.groupOrder=g,m.renderOrder=d.renderOrder,m.z=y,m.group=p),t++,m}function a(d,c,u,g,y,p){let m=o(d,c,u,g,y,p);u.transmission>0?i.push(m):u.transparent===!0?s.push(m):e.push(m)}function l(d,c,u,g,y,p){let m=o(d,c,u,g,y,p);u.transmission>0?i.unshift(m):u.transparent===!0?s.unshift(m):e.unshift(m)}function h(d,c){e.length>1&&e.sort(d||fx),i.length>1&&i.sort(c||Jh),s.length>1&&s.sort(c||Jh)}function f(){for(let d=t,c=n.length;d<c;d++){let u=n[d];if(u.id===null)break;u.id=null,u.object=null,u.geometry=null,u.material=null,u.group=null}}return{opaque:e,transmissive:i,transparent:s,init:r,push:a,unshift:l,finish:f,sort:h}}function px(){let n=new WeakMap;function t(i,s){let r=n.get(i),o;return r===void 0?(o=new Qh,n.set(i,[o])):s>=r.length?(o=new Qh,r.push(o)):o=r[s],o}function e(){n=new WeakMap}return{get:t,dispose:e}}function mx(){let n={};return{get:function(t){if(n[t.id]!==void 0)return n[t.id];let e;switch(t.type){case"DirectionalLight":e={direction:new U,color:new Ot};break;case"SpotLight":e={position:new U,direction:new U,color:new Ot,distance:0,coneCos:0,penumbraCos:0,decay:0};break;case"PointLight":e={position:new U,color:new Ot,distance:0,decay:0};break;case"HemisphereLight":e={direction:new U,skyColor:new Ot,groundColor:new Ot};break;case"RectAreaLight":e={color:new Ot,position:new U,halfWidth:new U,halfHeight:new U};break}return n[t.id]=e,e}}}function gx(){let n={};return{get:function(t){if(n[t.id]!==void 0)return n[t.id];let e;switch(t.type){case"DirectionalLight":e={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new yt};break;case"SpotLight":e={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new yt};break;case"PointLight":e={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new yt,shadowCameraNear:1,shadowCameraFar:1e3};break}return n[t.id]=e,e}}}var xx=0;function vx(n,t){return(t.castShadow?2:0)-(n.castShadow?2:0)+(t.map?1:0)-(n.map?1:0)}function _x(n){let t=new mx,e=gx(),i={version:0,hash:{directionalLength:-1,pointLength:-1,spotLength:-1,rectAreaLength:-1,hemiLength:-1,numDirectionalShadows:-1,numPointShadows:-1,numSpotShadows:-1,numSpotMaps:-1,numLightProbes:-1},ambient:[0,0,0],probe:[],directional:[],directionalShadow:[],directionalShadowMap:[],directionalShadowMatrix:[],spot:[],spotLightMap:[],spotShadow:[],spotShadowMap:[],spotLightMatrix:[],rectArea:[],rectAreaLTC1:null,rectAreaLTC2:null,point:[],pointShadow:[],pointShadowMap:[],pointShadowMatrix:[],hemi:[],numSpotLightShadowsWithMaps:0,numLightProbes:0};for(let h=0;h<9;h++)i.probe.push(new U);let s=new U,r=new $t,o=new $t;function a(h){let f=0,d=0,c=0;for(let H=0;H<9;H++)i.probe[H].set(0,0,0);let u=0,g=0,y=0,p=0,m=0,w=0,_=0,A=0,I=0,C=0,E=0;h.sort(vx);for(let H=0,x=h.length;H<x;H++){let S=h[H],O=S.color,W=S.intensity,q=S.distance,et=S.shadow&&S.shadow.map?S.shadow.map.texture:null;if(S.isAmbientLight)f+=O.r*W,d+=O.g*W,c+=O.b*W;else if(S.isLightProbe){for(let F=0;F<9;F++)i.probe[F].addScaledVector(S.sh.coefficients[F],W);E++}else if(S.isDirectionalLight){let F=t.get(S);if(F.color.copy(S.color).multiplyScalar(S.intensity),S.castShadow){let it=S.shadow,Z=e.get(S);Z.shadowIntensity=it.intensity,Z.shadowBias=it.bias,Z.shadowNormalBias=it.normalBias,Z.shadowRadius=it.radius,Z.shadowMapSize=it.mapSize,i.directionalShadow[u]=Z,i.directionalShadowMap[u]=et,i.directionalShadowMatrix[u]=S.shadow.matrix,w++}i.directional[u]=F,u++}else if(S.isSpotLight){let F=t.get(S);F.position.setFromMatrixPosition(S.matrixWorld),F.color.copy(O).multiplyScalar(W),F.distance=q,F.coneCos=Math.cos(S.angle),F.penumbraCos=Math.cos(S.angle*(1-S.penumbra)),F.decay=S.decay,i.spot[y]=F;let it=S.shadow;if(S.map&&(i.spotLightMap[I]=S.map,I++,it.updateMatrices(S),S.castShadow&&C++),i.spotLightMatrix[y]=it.matrix,S.castShadow){let Z=e.get(S);Z.shadowIntensity=it.intensity,Z.shadowBias=it.bias,Z.shadowNormalBias=it.normalBias,Z.shadowRadius=it.radius,Z.shadowMapSize=it.mapSize,i.spotShadow[y]=Z,i.spotShadowMap[y]=et,A++}y++}else if(S.isRectAreaLight){let F=t.get(S);F.color.copy(O).multiplyScalar(W),F.halfWidth.set(S.width*.5,0,0),F.halfHeight.set(0,S.height*.5,0),i.rectArea[p]=F,p++}else if(S.isPointLight){let F=t.get(S);if(F.color.copy(S.color).multiplyScalar(S.intensity),F.distance=S.distance,F.decay=S.decay,S.castShadow){let it=S.shadow,Z=e.get(S);Z.shadowIntensity=it.intensity,Z.shadowBias=it.bias,Z.shadowNormalBias=it.normalBias,Z.shadowRadius=it.radius,Z.shadowMapSize=it.mapSize,Z.shadowCameraNear=it.camera.near,Z.shadowCameraFar=it.camera.far,i.pointShadow[g]=Z,i.pointShadowMap[g]=et,i.pointShadowMatrix[g]=S.shadow.matrix,_++}i.point[g]=F,g++}else if(S.isHemisphereLight){let F=t.get(S);F.skyColor.copy(S.color).multiplyScalar(W),F.groundColor.copy(S.groundColor).multiplyScalar(W),i.hemi[m]=F,m++}}p>0&&(n.has("OES_texture_float_linear")===!0?(i.rectAreaLTC1=dt.LTC_FLOAT_1,i.rectAreaLTC2=dt.LTC_FLOAT_2):(i.rectAreaLTC1=dt.LTC_HALF_1,i.rectAreaLTC2=dt.LTC_HALF_2)),i.ambient[0]=f,i.ambient[1]=d,i.ambient[2]=c;let T=i.hash;(T.directionalLength!==u||T.pointLength!==g||T.spotLength!==y||T.rectAreaLength!==p||T.hemiLength!==m||T.numDirectionalShadows!==w||T.numPointShadows!==_||T.numSpotShadows!==A||T.numSpotMaps!==I||T.numLightProbes!==E)&&(i.directional.length=u,i.spot.length=y,i.rectArea.length=p,i.point.length=g,i.hemi.length=m,i.directionalShadow.length=w,i.directionalShadowMap.length=w,i.pointShadow.length=_,i.pointShadowMap.length=_,i.spotShadow.length=A,i.spotShadowMap.length=A,i.directionalShadowMatrix.length=w,i.pointShadowMatrix.length=_,i.spotLightMatrix.length=A+I-C,i.spotLightMap.length=I,i.numSpotLightShadowsWithMaps=C,i.numLightProbes=E,T.directionalLength=u,T.pointLength=g,T.spotLength=y,T.rectAreaLength=p,T.hemiLength=m,T.numDirectionalShadows=w,T.numPointShadows=_,T.numSpotShadows=A,T.numSpotMaps=I,T.numLightProbes=E,i.version=xx++)}function l(h,f){let d=0,c=0,u=0,g=0,y=0,p=f.matrixWorldInverse;for(let m=0,w=h.length;m<w;m++){let _=h[m];if(_.isDirectionalLight){let A=i.directional[d];A.direction.setFromMatrixPosition(_.matrixWorld),s.setFromMatrixPosition(_.target.matrixWorld),A.direction.sub(s),A.direction.transformDirection(p),d++}else if(_.isSpotLight){let A=i.spot[u];A.position.setFromMatrixPosition(_.matrixWorld),A.position.applyMatrix4(p),A.direction.setFromMatrixPosition(_.matrixWorld),s.setFromMatrixPosition(_.target.matrixWorld),A.direction.sub(s),A.direction.transformDirection(p),u++}else if(_.isRectAreaLight){let A=i.rectArea[g];A.position.setFromMatrixPosition(_.matrixWorld),A.position.applyMatrix4(p),o.identity(),r.copy(_.matrixWorld),r.premultiply(p),o.extractRotation(r),A.halfWidth.set(_.width*.5,0,0),A.halfHeight.set(0,_.height*.5,0),A.halfWidth.applyMatrix4(o),A.halfHeight.applyMatrix4(o),g++}else if(_.isPointLight){let A=i.point[c];A.position.setFromMatrixPosition(_.matrixWorld),A.position.applyMatrix4(p),c++}else if(_.isHemisphereLight){let A=i.hemi[y];A.direction.setFromMatrixPosition(_.matrixWorld),A.direction.transformDirection(p),y++}}}return{setup:a,setupView:l,state:i}}function $h(n){let t=new _x(n),e=[],i=[];function s(f){h.camera=f,e.length=0,i.length=0}function r(f){e.push(f)}function o(f){i.push(f)}function a(){t.setup(e)}function l(f){t.setupView(e,f)}let h={lightsArray:e,shadowsArray:i,camera:null,lights:t,transmissionRenderTarget:{}};return{init:s,state:h,setupLights:a,setupLightsView:l,pushLight:r,pushShadow:o}}function yx(n){let t=new WeakMap;function e(s,r=0){let o=t.get(s),a;return o===void 0?(a=new $h(n),t.set(s,[a])):r>=o.length?(a=new $h(n),o.push(a)):a=o[r],a}function i(){t=new WeakMap}return{get:e,dispose:i}}var zl=class extends yi{constructor(t){super(),this.isMeshDepthMaterial=!0,this.type="MeshDepthMaterial",this.depthPacking=df,this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.wireframe=!1,this.wireframeLinewidth=1,this.setValues(t)}copy(t){return super.copy(t),this.depthPacking=t.depthPacking,this.map=t.map,this.alphaMap=t.alphaMap,this.displacementMap=t.displacementMap,this.displacementScale=t.displacementScale,this.displacementBias=t.displacementBias,this.wireframe=t.wireframe,this.wireframeLinewidth=t.wireframeLinewidth,this}},kl=class extends yi{constructor(t){super(),this.isMeshDistanceMaterial=!0,this.type="MeshDistanceMaterial",this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.setValues(t)}copy(t){return super.copy(t),this.map=t.map,this.alphaMap=t.alphaMap,this.displacementMap=t.displacementMap,this.displacementScale=t.displacementScale,this.displacementBias=t.displacementBias,this}},Mx=`void main() {
	gl_Position = vec4( position, 1.0 );
}`,Sx=`uniform sampler2D shadow_pass;
uniform vec2 resolution;
uniform float radius;
#include <packing>
void main() {
	const float samples = float( VSM_SAMPLES );
	float mean = 0.0;
	float squared_mean = 0.0;
	float uvStride = samples <= 1.0 ? 0.0 : 2.0 / ( samples - 1.0 );
	float uvStart = samples <= 1.0 ? 0.0 : - 1.0;
	for ( float i = 0.0; i < samples; i ++ ) {
		float uvOffset = uvStart + i * uvStride;
		#ifdef HORIZONTAL_PASS
			vec2 distribution = unpackRGBATo2Half( texture2D( shadow_pass, ( gl_FragCoord.xy + vec2( uvOffset, 0.0 ) * radius ) / resolution ) );
			mean += distribution.x;
			squared_mean += distribution.y * distribution.y + distribution.x * distribution.x;
		#else
			float depth = unpackRGBAToDepth( texture2D( shadow_pass, ( gl_FragCoord.xy + vec2( 0.0, uvOffset ) * radius ) / resolution ) );
			mean += depth;
			squared_mean += depth * depth;
		#endif
	}
	mean = mean / samples;
	squared_mean = squared_mean / samples;
	float std_dev = sqrt( squared_mean - mean * mean );
	gl_FragColor = pack2HalfToRGBA( vec2( mean, std_dev ) );
}`;function bx(n,t,e){let i=new nr,s=new yt,r=new yt,o=new oe,a=new zl({depthPacking:ff}),l=new kl,h={},f=e.maxTextureSize,d={[jn]:Fe,[Fe]:jn,[Rn]:Rn},c=new ie({defines:{VSM_SAMPLES:8},uniforms:{shadow_pass:{value:null},resolution:{value:new yt},radius:{value:4}},vertexShader:Mx,fragmentShader:Sx}),u=c.clone();u.defines.HORIZONTAL_PASS=1;let g=new un;g.setAttribute("position",new qe(new Float32Array([-1,-1,.5,3,-1,.5,-1,3,.5]),3));let y=new Ue(g,c),p=this;this.enabled=!1,this.autoUpdate=!0,this.needsUpdate=!1,this.type=lu;let m=this.type;this.render=function(C,E,T){if(p.enabled===!1||p.autoUpdate===!1&&p.needsUpdate===!1||C.length===0)return;let H=n.getRenderTarget(),x=n.getActiveCubeFace(),S=n.getActiveMipmapLevel(),O=n.state;O.setBlending(vn),O.buffers.color.setClear(1,1,1,1),O.buffers.depth.setTest(!0),O.setScissorTest(!1);let W=m!==Cn&&this.type===Cn,q=m===Cn&&this.type!==Cn;for(let et=0,F=C.length;et<F;et++){let it=C[et],Z=it.shadow;if(Z===void 0){console.warn("THREE.WebGLShadowMap:",it,"has no shadow.");continue}if(Z.autoUpdate===!1&&Z.needsUpdate===!1)continue;s.copy(Z.mapSize);let gt=Z.getFrameExtents();if(s.multiply(gt),r.copy(Z.mapSize),(s.x>f||s.y>f)&&(s.x>f&&(r.x=Math.floor(f/gt.x),s.x=r.x*gt.x,Z.mapSize.x=r.x),s.y>f&&(r.y=Math.floor(f/gt.y),s.y=r.y*gt.y,Z.mapSize.y=r.y)),Z.map===null||W===!0||q===!0){let St=this.type!==Cn?{minFilter:be,magFilter:be}:{};Z.map!==null&&Z.map.dispose(),Z.map=new ue(s.x,s.y,St),Z.map.texture.name=it.name+".shadowMap",Z.camera.updateProjectionMatrix()}n.setRenderTarget(Z.map),n.clear();let _t=Z.getViewportCount();for(let St=0;St<_t;St++){let Gt=Z.getViewport(St);o.set(r.x*Gt.x,r.y*Gt.y,r.x*Gt.z,r.y*Gt.w),O.viewport(o),Z.updateMatrices(it,St),i=Z.getFrustum(),A(E,T,Z.camera,it,this.type)}Z.isPointLightShadow!==!0&&this.type===Cn&&w(Z,T),Z.needsUpdate=!1}m=this.type,p.needsUpdate=!1,n.setRenderTarget(H,x,S)};function w(C,E){let T=t.update(y);c.defines.VSM_SAMPLES!==C.blurSamples&&(c.defines.VSM_SAMPLES=C.blurSamples,u.defines.VSM_SAMPLES=C.blurSamples,c.needsUpdate=!0,u.needsUpdate=!0),C.mapPass===null&&(C.mapPass=new ue(s.x,s.y)),c.uniforms.shadow_pass.value=C.map.texture,c.uniforms.resolution.value=C.mapSize,c.uniforms.radius.value=C.radius,n.setRenderTarget(C.mapPass),n.clear(),n.renderBufferDirect(E,null,T,c,y,null),u.uniforms.shadow_pass.value=C.mapPass.texture,u.uniforms.resolution.value=C.mapSize,u.uniforms.radius.value=C.radius,n.setRenderTarget(C.map),n.clear(),n.renderBufferDirect(E,null,T,u,y,null)}function _(C,E,T,H){let x=null,S=T.isPointLight===!0?C.customDistanceMaterial:C.customDepthMaterial;if(S!==void 0)x=S;else if(x=T.isPointLight===!0?l:a,n.localClippingEnabled&&E.clipShadows===!0&&Array.isArray(E.clippingPlanes)&&E.clippingPlanes.length!==0||E.displacementMap&&E.displacementScale!==0||E.alphaMap&&E.alphaTest>0||E.map&&E.alphaTest>0){let O=x.uuid,W=E.uuid,q=h[O];q===void 0&&(q={},h[O]=q);let et=q[W];et===void 0&&(et=x.clone(),q[W]=et,E.addEventListener("dispose",I)),x=et}if(x.visible=E.visible,x.wireframe=E.wireframe,H===Cn?x.side=E.shadowSide!==null?E.shadowSide:E.side:x.side=E.shadowSide!==null?E.shadowSide:d[E.side],x.alphaMap=E.alphaMap,x.alphaTest=E.alphaTest,x.map=E.map,x.clipShadows=E.clipShadows,x.clippingPlanes=E.clippingPlanes,x.clipIntersection=E.clipIntersection,x.displacementMap=E.displacementMap,x.displacementScale=E.displacementScale,x.displacementBias=E.displacementBias,x.wireframeLinewidth=E.wireframeLinewidth,x.linewidth=E.linewidth,T.isPointLight===!0&&x.isMeshDistanceMaterial===!0){let O=n.properties.get(x);O.light=T}return x}function A(C,E,T,H,x){if(C.visible===!1)return;if(C.layers.test(E.layers)&&(C.isMesh||C.isLine||C.isPoints)&&(C.castShadow||C.receiveShadow&&x===Cn)&&(!C.frustumCulled||i.intersectsObject(C))){C.modelViewMatrix.multiplyMatrices(T.matrixWorldInverse,C.matrixWorld);let W=t.update(C),q=C.material;if(Array.isArray(q)){let et=W.groups;for(let F=0,it=et.length;F<it;F++){let Z=et[F],gt=q[Z.materialIndex];if(gt&&gt.visible){let _t=_(C,gt,H,x);C.onBeforeShadow(n,C,E,T,W,_t,Z),n.renderBufferDirect(T,null,W,_t,C,Z),C.onAfterShadow(n,C,E,T,W,_t,Z)}}}else if(q.visible){let et=_(C,q,H,x);C.onBeforeShadow(n,C,E,T,W,et,null),n.renderBufferDirect(T,null,W,et,C,null),C.onAfterShadow(n,C,E,T,W,et,null)}}let O=C.children;for(let W=0,q=O.length;W<q;W++)A(O[W],E,T,H,x)}function I(C){C.target.removeEventListener("dispose",I);for(let T in h){let H=h[T],x=C.target.uuid;x in H&&(H[x].dispose(),delete H[x])}}}var wx={[Wa]:Xa,[qa]:ja,[Ya]:Ka,[ls]:Za,[Xa]:Wa,[ja]:qa,[Ka]:Ya,[Za]:ls};function Ax(n){function t(){let L=!1,ft=new oe,Y=null,$=new oe(0,0,0,0);return{setMask:function(pt){Y!==pt&&!L&&(n.colorMask(pt,pt,pt,pt),Y=pt)},setLocked:function(pt){L=pt},setClear:function(pt,vt,Xt,ce,Ne){Ne===!0&&(pt*=ce,vt*=ce,Xt*=ce),ft.set(pt,vt,Xt,ce),$.equals(ft)===!1&&(n.clearColor(pt,vt,Xt,ce),$.copy(ft))},reset:function(){L=!1,Y=null,$.set(-1,0,0,0)}}}function e(){let L=!1,ft=!1,Y=null,$=null,pt=null;return{setReversed:function(vt){ft=vt},setTest:function(vt){vt?Mt(n.DEPTH_TEST):xt(n.DEPTH_TEST)},setMask:function(vt){Y!==vt&&!L&&(n.depthMask(vt),Y=vt)},setFunc:function(vt){if(ft&&(vt=wx[vt]),$!==vt){switch(vt){case Wa:n.depthFunc(n.NEVER);break;case Xa:n.depthFunc(n.ALWAYS);break;case qa:n.depthFunc(n.LESS);break;case ls:n.depthFunc(n.LEQUAL);break;case Ya:n.depthFunc(n.EQUAL);break;case Za:n.depthFunc(n.GEQUAL);break;case ja:n.depthFunc(n.GREATER);break;case Ka:n.depthFunc(n.NOTEQUAL);break;default:n.depthFunc(n.LEQUAL)}$=vt}},setLocked:function(vt){L=vt},setClear:function(vt){pt!==vt&&(n.clearDepth(vt),pt=vt)},reset:function(){L=!1,Y=null,$=null,pt=null}}}function i(){let L=!1,ft=null,Y=null,$=null,pt=null,vt=null,Xt=null,ce=null,Ne=null;return{setTest:function(qt){L||(qt?Mt(n.STENCIL_TEST):xt(n.STENCIL_TEST))},setMask:function(qt){ft!==qt&&!L&&(n.stencilMask(qt),ft=qt)},setFunc:function(qt,Oe,Sn){(Y!==qt||$!==Oe||pt!==Sn)&&(n.stencilFunc(qt,Oe,Sn),Y=qt,$=Oe,pt=Sn)},setOp:function(qt,Oe,Sn){(vt!==qt||Xt!==Oe||ce!==Sn)&&(n.stencilOp(qt,Oe,Sn),vt=qt,Xt=Oe,ce=Sn)},setLocked:function(qt){L=qt},setClear:function(qt){Ne!==qt&&(n.clearStencil(qt),Ne=qt)},reset:function(){L=!1,ft=null,Y=null,$=null,pt=null,vt=null,Xt=null,ce=null,Ne=null}}}let s=new t,r=new e,o=new i,a=new WeakMap,l=new WeakMap,h={},f={},d=new WeakMap,c=[],u=null,g=!1,y=null,p=null,m=null,w=null,_=null,A=null,I=null,C=new Ot(0,0,0),E=0,T=!1,H=null,x=null,S=null,O=null,W=null,q=n.getParameter(n.MAX_COMBINED_TEXTURE_IMAGE_UNITS),et=!1,F=0,it=n.getParameter(n.VERSION);it.indexOf("WebGL")!==-1?(F=parseFloat(/^WebGL (\d)/.exec(it)[1]),et=F>=1):it.indexOf("OpenGL ES")!==-1&&(F=parseFloat(/^OpenGL ES (\d)/.exec(it)[1]),et=F>=2);let Z=null,gt={},_t=n.getParameter(n.SCISSOR_BOX),St=n.getParameter(n.VIEWPORT),Gt=new oe().fromArray(_t),Wt=new oe().fromArray(St);function J(L,ft,Y,$){let pt=new Uint8Array(4),vt=n.createTexture();n.bindTexture(L,vt),n.texParameteri(L,n.TEXTURE_MIN_FILTER,n.NEAREST),n.texParameteri(L,n.TEXTURE_MAG_FILTER,n.NEAREST);for(let Xt=0;Xt<Y;Xt++)L===n.TEXTURE_3D||L===n.TEXTURE_2D_ARRAY?n.texImage3D(ft,0,n.RGBA,1,1,$,0,n.RGBA,n.UNSIGNED_BYTE,pt):n.texImage2D(ft+Xt,0,n.RGBA,1,1,0,n.RGBA,n.UNSIGNED_BYTE,pt);return vt}let st={};st[n.TEXTURE_2D]=J(n.TEXTURE_2D,n.TEXTURE_2D,1),st[n.TEXTURE_CUBE_MAP]=J(n.TEXTURE_CUBE_MAP,n.TEXTURE_CUBE_MAP_POSITIVE_X,6),st[n.TEXTURE_2D_ARRAY]=J(n.TEXTURE_2D_ARRAY,n.TEXTURE_2D_ARRAY,1,1),st[n.TEXTURE_3D]=J(n.TEXTURE_3D,n.TEXTURE_3D,1,1),s.setClear(0,0,0,1),r.setClear(1),o.setClear(0),Mt(n.DEPTH_TEST),r.setFunc(ls),j(!1),lt(rh),Mt(n.CULL_FACE),P(vn);function Mt(L){h[L]!==!0&&(n.enable(L),h[L]=!0)}function xt(L){h[L]!==!1&&(n.disable(L),h[L]=!1)}function Nt(L,ft){return f[L]!==ft?(n.bindFramebuffer(L,ft),f[L]=ft,L===n.DRAW_FRAMEBUFFER&&(f[n.FRAMEBUFFER]=ft),L===n.FRAMEBUFFER&&(f[n.DRAW_FRAMEBUFFER]=ft),!0):!1}function It(L,ft){let Y=c,$=!1;if(L){Y=d.get(ft),Y===void 0&&(Y=[],d.set(ft,Y));let pt=L.textures;if(Y.length!==pt.length||Y[0]!==n.COLOR_ATTACHMENT0){for(let vt=0,Xt=pt.length;vt<Xt;vt++)Y[vt]=n.COLOR_ATTACHMENT0+vt;Y.length=pt.length,$=!0}}else Y[0]!==n.BACK&&(Y[0]=n.BACK,$=!0);$&&n.drawBuffers(Y)}function kt(L){return u!==L?(n.useProgram(L),u=L,!0):!1}let Zt={[fi]:n.FUNC_ADD,[Bd]:n.FUNC_SUBTRACT,[zd]:n.FUNC_REVERSE_SUBTRACT};Zt[kd]=n.MIN,Zt[Hd]=n.MAX;let Vt={[Vd]:n.ZERO,[Gd]:n.ONE,[Wd]:n.SRC_COLOR,[Va]:n.SRC_ALPHA,[Kd]:n.SRC_ALPHA_SATURATE,[Zd]:n.DST_COLOR,[qd]:n.DST_ALPHA,[Xd]:n.ONE_MINUS_SRC_COLOR,[Ga]:n.ONE_MINUS_SRC_ALPHA,[jd]:n.ONE_MINUS_DST_COLOR,[Yd]:n.ONE_MINUS_DST_ALPHA,[Jd]:n.CONSTANT_COLOR,[Qd]:n.ONE_MINUS_CONSTANT_COLOR,[$d]:n.CONSTANT_ALPHA,[tf]:n.ONE_MINUS_CONSTANT_ALPHA};function P(L,ft,Y,$,pt,vt,Xt,ce,Ne,qt){if(L===vn){g===!0&&(xt(n.BLEND),g=!1);return}if(g===!1&&(Mt(n.BLEND),g=!0),L!==Fd){if(L!==y||qt!==T){if((p!==fi||_!==fi)&&(n.blendEquation(n.FUNC_ADD),p=fi,_=fi),qt)switch(L){case is:n.blendFuncSeparate(n.ONE,n.ONE_MINUS_SRC_ALPHA,n.ONE,n.ONE_MINUS_SRC_ALPHA);break;case as:n.blendFunc(n.ONE,n.ONE);break;case oh:n.blendFuncSeparate(n.ZERO,n.ONE_MINUS_SRC_COLOR,n.ZERO,n.ONE);break;case ah:n.blendFuncSeparate(n.ZERO,n.SRC_COLOR,n.ZERO,n.SRC_ALPHA);break;default:console.error("THREE.WebGLState: Invalid blending: ",L);break}else switch(L){case is:n.blendFuncSeparate(n.SRC_ALPHA,n.ONE_MINUS_SRC_ALPHA,n.ONE,n.ONE_MINUS_SRC_ALPHA);break;case as:n.blendFunc(n.SRC_ALPHA,n.ONE);break;case oh:n.blendFuncSeparate(n.ZERO,n.ONE_MINUS_SRC_COLOR,n.ZERO,n.ONE);break;case ah:n.blendFunc(n.ZERO,n.SRC_COLOR);break;default:console.error("THREE.WebGLState: Invalid blending: ",L);break}m=null,w=null,A=null,I=null,C.set(0,0,0),E=0,y=L,T=qt}return}pt=pt||ft,vt=vt||Y,Xt=Xt||$,(ft!==p||pt!==_)&&(n.blendEquationSeparate(Zt[ft],Zt[pt]),p=ft,_=pt),(Y!==m||$!==w||vt!==A||Xt!==I)&&(n.blendFuncSeparate(Vt[Y],Vt[$],Vt[vt],Vt[Xt]),m=Y,w=$,A=vt,I=Xt),(ce.equals(C)===!1||Ne!==E)&&(n.blendColor(ce.r,ce.g,ce.b,Ne),C.copy(ce),E=Ne),y=L,T=!1}function fe(L,ft){L.side===Rn?xt(n.CULL_FACE):Mt(n.CULL_FACE);let Y=L.side===Fe;ft&&(Y=!Y),j(Y),L.blending===is&&L.transparent===!1?P(vn):P(L.blending,L.blendEquation,L.blendSrc,L.blendDst,L.blendEquationAlpha,L.blendSrcAlpha,L.blendDstAlpha,L.blendColor,L.blendAlpha,L.premultipliedAlpha),r.setFunc(L.depthFunc),r.setTest(L.depthTest),r.setMask(L.depthWrite),s.setMask(L.colorWrite);let $=L.stencilWrite;o.setTest($),$&&(o.setMask(L.stencilWriteMask),o.setFunc(L.stencilFunc,L.stencilRef,L.stencilFuncMask),o.setOp(L.stencilFail,L.stencilZFail,L.stencilZPass)),Rt(L.polygonOffset,L.polygonOffsetFactor,L.polygonOffsetUnits),L.alphaToCoverage===!0?Mt(n.SAMPLE_ALPHA_TO_COVERAGE):xt(n.SAMPLE_ALPHA_TO_COVERAGE)}function j(L){H!==L&&(L?n.frontFace(n.CW):n.frontFace(n.CCW),H=L)}function lt(L){L!==Ud?(Mt(n.CULL_FACE),L!==x&&(L===rh?n.cullFace(n.BACK):L===Nd?n.cullFace(n.FRONT):n.cullFace(n.FRONT_AND_BACK))):xt(n.CULL_FACE),x=L}function ut(L){L!==S&&(et&&n.lineWidth(L),S=L)}function Rt(L,ft,Y){L?(Mt(n.POLYGON_OFFSET_FILL),(O!==ft||W!==Y)&&(n.polygonOffset(ft,Y),O=ft,W=Y)):xt(n.POLYGON_OFFSET_FILL)}function B(L){L?Mt(n.SCISSOR_TEST):xt(n.SCISSOR_TEST)}function M(L){L===void 0&&(L=n.TEXTURE0+q-1),Z!==L&&(n.activeTexture(L),Z=L)}function v(L,ft,Y){Y===void 0&&(Z===null?Y=n.TEXTURE0+q-1:Y=Z);let $=gt[Y];$===void 0&&($={type:void 0,texture:void 0},gt[Y]=$),($.type!==L||$.texture!==ft)&&(Z!==Y&&(n.activeTexture(Y),Z=Y),n.bindTexture(L,ft||st[L]),$.type=L,$.texture=ft)}function R(){let L=gt[Z];L!==void 0&&L.type!==void 0&&(n.bindTexture(L.type,null),L.type=void 0,L.texture=void 0)}function k(){try{n.compressedTexImage2D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function K(){try{n.compressedTexImage3D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function z(){try{n.texSubImage2D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function nt(){try{n.texSubImage3D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function Q(){try{n.compressedTexSubImage2D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function X(){try{n.compressedTexSubImage3D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function ht(){try{n.texStorage2D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function tt(){try{n.texStorage3D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function ct(){try{n.texImage2D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function At(){try{n.texImage3D.apply(n,arguments)}catch(L){console.error("THREE.WebGLState:",L)}}function rt(L){Gt.equals(L)===!1&&(n.scissor(L.x,L.y,L.z,L.w),Gt.copy(L))}function ot(L){Wt.equals(L)===!1&&(n.viewport(L.x,L.y,L.z,L.w),Wt.copy(L))}function Lt(L,ft){let Y=l.get(ft);Y===void 0&&(Y=new WeakMap,l.set(ft,Y));let $=Y.get(L);$===void 0&&($=n.getUniformBlockIndex(ft,L.name),Y.set(L,$))}function wt(L,ft){let $=l.get(ft).get(L);a.get(ft)!==$&&(n.uniformBlockBinding(ft,$,L.__bindingPointIndex),a.set(ft,$))}function Ft(){n.disable(n.BLEND),n.disable(n.CULL_FACE),n.disable(n.DEPTH_TEST),n.disable(n.POLYGON_OFFSET_FILL),n.disable(n.SCISSOR_TEST),n.disable(n.STENCIL_TEST),n.disable(n.SAMPLE_ALPHA_TO_COVERAGE),n.blendEquation(n.FUNC_ADD),n.blendFunc(n.ONE,n.ZERO),n.blendFuncSeparate(n.ONE,n.ZERO,n.ONE,n.ZERO),n.blendColor(0,0,0,0),n.colorMask(!0,!0,!0,!0),n.clearColor(0,0,0,0),n.depthMask(!0),n.depthFunc(n.LESS),n.clearDepth(1),n.stencilMask(4294967295),n.stencilFunc(n.ALWAYS,0,4294967295),n.stencilOp(n.KEEP,n.KEEP,n.KEEP),n.clearStencil(0),n.cullFace(n.BACK),n.frontFace(n.CCW),n.polygonOffset(0,0),n.activeTexture(n.TEXTURE0),n.bindFramebuffer(n.FRAMEBUFFER,null),n.bindFramebuffer(n.DRAW_FRAMEBUFFER,null),n.bindFramebuffer(n.READ_FRAMEBUFFER,null),n.useProgram(null),n.lineWidth(1),n.scissor(0,0,n.canvas.width,n.canvas.height),n.viewport(0,0,n.canvas.width,n.canvas.height),h={},Z=null,gt={},f={},d=new WeakMap,c=[],u=null,g=!1,y=null,p=null,m=null,w=null,_=null,A=null,I=null,C=new Ot(0,0,0),E=0,T=!1,H=null,x=null,S=null,O=null,W=null,Gt.set(0,0,n.canvas.width,n.canvas.height),Wt.set(0,0,n.canvas.width,n.canvas.height),s.reset(),r.reset(),o.reset()}return{buffers:{color:s,depth:r,stencil:o},enable:Mt,disable:xt,bindFramebuffer:Nt,drawBuffers:It,useProgram:kt,setBlending:P,setMaterial:fe,setFlipSided:j,setCullFace:lt,setLineWidth:ut,setPolygonOffset:Rt,setScissorTest:B,activeTexture:M,bindTexture:v,unbindTexture:R,compressedTexImage2D:k,compressedTexImage3D:K,texImage2D:ct,texImage3D:At,updateUBOMapping:Lt,uniformBlockBinding:wt,texStorage2D:ht,texStorage3D:tt,texSubImage2D:z,texSubImage3D:nt,compressedTexSubImage2D:Q,compressedTexSubImage3D:X,scissor:rt,viewport:ot,reset:Ft}}function tu(n,t,e,i){let s=Ex(i);switch(e){case pu:return n*t;case gu:return n*t;case xu:return n*t*2;case lc:return n*t/s.components*s.byteLength;case cc:return n*t/s.components*s.byteLength;case vu:return n*t*2/s.components*s.byteLength;case hc:return n*t*2/s.components*s.byteLength;case mu:return n*t*3/s.components*s.byteLength;case hn:return n*t*4/s.components*s.byteLength;case uc:return n*t*4/s.components*s.byteLength;case Gr:case Wr:return Math.floor((n+3)/4)*Math.floor((t+3)/4)*8;case Xr:case qr:return Math.floor((n+3)/4)*Math.floor((t+3)/4)*16;case nl:case sl:return Math.max(n,16)*Math.max(t,8)/4;case el:case il:return Math.max(n,8)*Math.max(t,8)/2;case rl:case ol:return Math.floor((n+3)/4)*Math.floor((t+3)/4)*8;case al:return Math.floor((n+3)/4)*Math.floor((t+3)/4)*16;case ll:return Math.floor((n+3)/4)*Math.floor((t+3)/4)*16;case cl:return Math.floor((n+4)/5)*Math.floor((t+3)/4)*16;case hl:return Math.floor((n+4)/5)*Math.floor((t+4)/5)*16;case ul:return Math.floor((n+5)/6)*Math.floor((t+4)/5)*16;case dl:return Math.floor((n+5)/6)*Math.floor((t+5)/6)*16;case fl:return Math.floor((n+7)/8)*Math.floor((t+4)/5)*16;case pl:return Math.floor((n+7)/8)*Math.floor((t+5)/6)*16;case ml:return Math.floor((n+7)/8)*Math.floor((t+7)/8)*16;case gl:return Math.floor((n+9)/10)*Math.floor((t+4)/5)*16;case xl:return Math.floor((n+9)/10)*Math.floor((t+5)/6)*16;case vl:return Math.floor((n+9)/10)*Math.floor((t+7)/8)*16;case _l:return Math.floor((n+9)/10)*Math.floor((t+9)/10)*16;case yl:return Math.floor((n+11)/12)*Math.floor((t+9)/10)*16;case Ml:return Math.floor((n+11)/12)*Math.floor((t+11)/12)*16;case Yr:case Sl:case bl:return Math.ceil(n/4)*Math.ceil(t/4)*16;case _u:case wl:return Math.ceil(n/4)*Math.ceil(t/4)*8;case Al:case El:return Math.ceil(n/4)*Math.ceil(t/4)*16}throw new Error(`Unable to determine texture byte length for ${e} format.`)}function Ex(n){switch(n){case In:case uu:return{byteLength:1,components:1};case Qs:case du:case ze:return{byteLength:2,components:1};case oc:case ac:return{byteLength:2,components:4};case vi:case rc:case xn:return{byteLength:4,components:1};case fu:return{byteLength:4,components:3}}throw new Error(`Unknown texture type ${n}.`)}function Tx(n,t,e,i,s,r,o){let a=t.has("WEBGL_multisampled_render_to_texture")?t.get("WEBGL_multisampled_render_to_texture"):null,l=typeof navigator>"u"?!1:/OculusBrowser/g.test(navigator.userAgent),h=new yt,f=new WeakMap,d,c=new WeakMap,u=!1;try{u=typeof OffscreenCanvas<"u"&&new OffscreenCanvas(1,1).getContext("2d")!==null}catch{}function g(M,v){return u?new OffscreenCanvas(M,v):to("canvas")}function y(M,v,R){let k=1,K=B(M);if((K.width>R||K.height>R)&&(k=R/Math.max(K.width,K.height)),k<1)if(typeof HTMLImageElement<"u"&&M instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&M instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&M instanceof ImageBitmap||typeof VideoFrame<"u"&&M instanceof VideoFrame){let z=Math.floor(k*K.width),nt=Math.floor(k*K.height);d===void 0&&(d=g(z,nt));let Q=v?g(z,nt):d;return Q.width=z,Q.height=nt,Q.getContext("2d").drawImage(M,0,0,z,nt),console.warn("THREE.WebGLRenderer: Texture has been resized from ("+K.width+"x"+K.height+") to ("+z+"x"+nt+")."),Q}else return"data"in M&&console.warn("THREE.WebGLRenderer: Image in DataTexture is too big ("+K.width+"x"+K.height+")."),M;return M}function p(M){return M.generateMipmaps&&M.minFilter!==be&&M.minFilter!==Xe}function m(M){n.generateMipmap(M)}function w(M,v,R,k,K=!1){if(M!==null){if(n[M]!==void 0)return n[M];console.warn("THREE.WebGLRenderer: Attempt to use non-existing WebGL internal format '"+M+"'")}let z=v;if(v===n.RED&&(R===n.FLOAT&&(z=n.R32F),R===n.HALF_FLOAT&&(z=n.R16F),R===n.UNSIGNED_BYTE&&(z=n.R8)),v===n.RED_INTEGER&&(R===n.UNSIGNED_BYTE&&(z=n.R8UI),R===n.UNSIGNED_SHORT&&(z=n.R16UI),R===n.UNSIGNED_INT&&(z=n.R32UI),R===n.BYTE&&(z=n.R8I),R===n.SHORT&&(z=n.R16I),R===n.INT&&(z=n.R32I)),v===n.RG&&(R===n.FLOAT&&(z=n.RG32F),R===n.HALF_FLOAT&&(z=n.RG16F),R===n.UNSIGNED_BYTE&&(z=n.RG8)),v===n.RG_INTEGER&&(R===n.UNSIGNED_BYTE&&(z=n.RG8UI),R===n.UNSIGNED_SHORT&&(z=n.RG16UI),R===n.UNSIGNED_INT&&(z=n.RG32UI),R===n.BYTE&&(z=n.RG8I),R===n.SHORT&&(z=n.RG16I),R===n.INT&&(z=n.RG32I)),v===n.RGB_INTEGER&&(R===n.UNSIGNED_BYTE&&(z=n.RGB8UI),R===n.UNSIGNED_SHORT&&(z=n.RGB16UI),R===n.UNSIGNED_INT&&(z=n.RGB32UI),R===n.BYTE&&(z=n.RGB8I),R===n.SHORT&&(z=n.RGB16I),R===n.INT&&(z=n.RGB32I)),v===n.RGBA_INTEGER&&(R===n.UNSIGNED_BYTE&&(z=n.RGBA8UI),R===n.UNSIGNED_SHORT&&(z=n.RGBA16UI),R===n.UNSIGNED_INT&&(z=n.RGBA32UI),R===n.BYTE&&(z=n.RGBA8I),R===n.SHORT&&(z=n.RGBA16I),R===n.INT&&(z=n.RGBA32I)),v===n.RGB&&R===n.UNSIGNED_INT_5_9_9_9_REV&&(z=n.RGB9_E5),v===n.RGBA){let nt=K?Kr:Jt.getTransfer(k);R===n.FLOAT&&(z=n.RGBA32F),R===n.HALF_FLOAT&&(z=n.RGBA16F),R===n.UNSIGNED_BYTE&&(z=nt===ee?n.SRGB8_ALPHA8:n.RGBA8),R===n.UNSIGNED_SHORT_4_4_4_4&&(z=n.RGBA4),R===n.UNSIGNED_SHORT_5_5_5_1&&(z=n.RGB5_A1)}return(z===n.R16F||z===n.R32F||z===n.RG16F||z===n.RG32F||z===n.RGBA16F||z===n.RGBA32F)&&t.get("EXT_color_buffer_float"),z}function _(M,v){let R;return M?v===null||v===vi||v===us?R=n.DEPTH24_STENCIL8:v===xn?R=n.DEPTH32F_STENCIL8:v===Qs&&(R=n.DEPTH24_STENCIL8,console.warn("DepthTexture: 16 bit depth attachment is not supported with stencil. Using 24-bit attachment.")):v===null||v===vi||v===us?R=n.DEPTH_COMPONENT24:v===xn?R=n.DEPTH_COMPONENT32F:v===Qs&&(R=n.DEPTH_COMPONENT16),R}function A(M,v){return p(M)===!0||M.isFramebufferTexture&&M.minFilter!==be&&M.minFilter!==Xe?Math.log2(Math.max(v.width,v.height))+1:M.mipmaps!==void 0&&M.mipmaps.length>0?M.mipmaps.length:M.isCompressedTexture&&Array.isArray(M.image)?v.mipmaps.length:1}function I(M){let v=M.target;v.removeEventListener("dispose",I),E(v),v.isVideoTexture&&f.delete(v)}function C(M){let v=M.target;v.removeEventListener("dispose",C),H(v)}function E(M){let v=i.get(M);if(v.__webglInit===void 0)return;let R=M.source,k=c.get(R);if(k){let K=k[v.__cacheKey];K.usedTimes--,K.usedTimes===0&&T(M),Object.keys(k).length===0&&c.delete(R)}i.remove(M)}function T(M){let v=i.get(M);n.deleteTexture(v.__webglTexture);let R=M.source,k=c.get(R);delete k[v.__cacheKey],o.memory.textures--}function H(M){let v=i.get(M);if(M.depthTexture&&M.depthTexture.dispose(),M.isWebGLCubeRenderTarget)for(let k=0;k<6;k++){if(Array.isArray(v.__webglFramebuffer[k]))for(let K=0;K<v.__webglFramebuffer[k].length;K++)n.deleteFramebuffer(v.__webglFramebuffer[k][K]);else n.deleteFramebuffer(v.__webglFramebuffer[k]);v.__webglDepthbuffer&&n.deleteRenderbuffer(v.__webglDepthbuffer[k])}else{if(Array.isArray(v.__webglFramebuffer))for(let k=0;k<v.__webglFramebuffer.length;k++)n.deleteFramebuffer(v.__webglFramebuffer[k]);else n.deleteFramebuffer(v.__webglFramebuffer);if(v.__webglDepthbuffer&&n.deleteRenderbuffer(v.__webglDepthbuffer),v.__webglMultisampledFramebuffer&&n.deleteFramebuffer(v.__webglMultisampledFramebuffer),v.__webglColorRenderbuffer)for(let k=0;k<v.__webglColorRenderbuffer.length;k++)v.__webglColorRenderbuffer[k]&&n.deleteRenderbuffer(v.__webglColorRenderbuffer[k]);v.__webglDepthRenderbuffer&&n.deleteRenderbuffer(v.__webglDepthRenderbuffer)}let R=M.textures;for(let k=0,K=R.length;k<K;k++){let z=i.get(R[k]);z.__webglTexture&&(n.deleteTexture(z.__webglTexture),o.memory.textures--),i.remove(R[k])}i.remove(M)}let x=0;function S(){x=0}function O(){let M=x;return M>=s.maxTextures&&console.warn("THREE.WebGLTextures: Trying to use "+M+" texture units while this GPU supports only "+s.maxTextures),x+=1,M}function W(M){let v=[];return v.push(M.wrapS),v.push(M.wrapT),v.push(M.wrapR||0),v.push(M.magFilter),v.push(M.minFilter),v.push(M.anisotropy),v.push(M.internalFormat),v.push(M.format),v.push(M.type),v.push(M.generateMipmaps),v.push(M.premultiplyAlpha),v.push(M.flipY),v.push(M.unpackAlignment),v.push(M.colorSpace),v.join()}function q(M,v){let R=i.get(M);if(M.isVideoTexture&&ut(M),M.isRenderTargetTexture===!1&&M.version>0&&R.__version!==M.version){let k=M.image;if(k===null)console.warn("THREE.WebGLRenderer: Texture marked for update but no image data found.");else if(k.complete===!1)console.warn("THREE.WebGLRenderer: Texture marked for update but image is incomplete");else{Wt(R,M,v);return}}e.bindTexture(n.TEXTURE_2D,R.__webglTexture,n.TEXTURE0+v)}function et(M,v){let R=i.get(M);if(M.version>0&&R.__version!==M.version){Wt(R,M,v);return}e.bindTexture(n.TEXTURE_2D_ARRAY,R.__webglTexture,n.TEXTURE0+v)}function F(M,v){let R=i.get(M);if(M.version>0&&R.__version!==M.version){Wt(R,M,v);return}e.bindTexture(n.TEXTURE_3D,R.__webglTexture,n.TEXTURE0+v)}function it(M,v){let R=i.get(M);if(M.version>0&&R.__version!==M.version){J(R,M,v);return}e.bindTexture(n.TEXTURE_CUBE_MAP,R.__webglTexture,n.TEXTURE0+v)}let Z={[$a]:n.REPEAT,[gi]:n.CLAMP_TO_EDGE,[tl]:n.MIRRORED_REPEAT},gt={[be]:n.NEAREST,[uf]:n.NEAREST_MIPMAP_NEAREST,[yr]:n.NEAREST_MIPMAP_LINEAR,[Xe]:n.LINEAR,[da]:n.LINEAR_MIPMAP_NEAREST,[xi]:n.LINEAR_MIPMAP_LINEAR},_t={[mf]:n.NEVER,[Mf]:n.ALWAYS,[gf]:n.LESS,[Mu]:n.LEQUAL,[xf]:n.EQUAL,[yf]:n.GEQUAL,[vf]:n.GREATER,[_f]:n.NOTEQUAL};function St(M,v){if(v.type===xn&&t.has("OES_texture_float_linear")===!1&&(v.magFilter===Xe||v.magFilter===da||v.magFilter===yr||v.magFilter===xi||v.minFilter===Xe||v.minFilter===da||v.minFilter===yr||v.minFilter===xi)&&console.warn("THREE.WebGLRenderer: Unable to use linear filtering with floating point textures. OES_texture_float_linear not supported on this device."),n.texParameteri(M,n.TEXTURE_WRAP_S,Z[v.wrapS]),n.texParameteri(M,n.TEXTURE_WRAP_T,Z[v.wrapT]),(M===n.TEXTURE_3D||M===n.TEXTURE_2D_ARRAY)&&n.texParameteri(M,n.TEXTURE_WRAP_R,Z[v.wrapR]),n.texParameteri(M,n.TEXTURE_MAG_FILTER,gt[v.magFilter]),n.texParameteri(M,n.TEXTURE_MIN_FILTER,gt[v.minFilter]),v.compareFunction&&(n.texParameteri(M,n.TEXTURE_COMPARE_MODE,n.COMPARE_REF_TO_TEXTURE),n.texParameteri(M,n.TEXTURE_COMPARE_FUNC,_t[v.compareFunction])),t.has("EXT_texture_filter_anisotropic")===!0){if(v.magFilter===be||v.minFilter!==yr&&v.minFilter!==xi||v.type===xn&&t.has("OES_texture_float_linear")===!1)return;if(v.anisotropy>1||i.get(v).__currentAnisotropy){let R=t.get("EXT_texture_filter_anisotropic");n.texParameterf(M,R.TEXTURE_MAX_ANISOTROPY_EXT,Math.min(v.anisotropy,s.getMaxAnisotropy())),i.get(v).__currentAnisotropy=v.anisotropy}}}function Gt(M,v){let R=!1;M.__webglInit===void 0&&(M.__webglInit=!0,v.addEventListener("dispose",I));let k=v.source,K=c.get(k);K===void 0&&(K={},c.set(k,K));let z=W(v);if(z!==M.__cacheKey){K[z]===void 0&&(K[z]={texture:n.createTexture(),usedTimes:0},o.memory.textures++,R=!0),K[z].usedTimes++;let nt=K[M.__cacheKey];nt!==void 0&&(K[M.__cacheKey].usedTimes--,nt.usedTimes===0&&T(v)),M.__cacheKey=z,M.__webglTexture=K[z].texture}return R}function Wt(M,v,R){let k=n.TEXTURE_2D;(v.isDataArrayTexture||v.isCompressedArrayTexture)&&(k=n.TEXTURE_2D_ARRAY),v.isData3DTexture&&(k=n.TEXTURE_3D);let K=Gt(M,v),z=v.source;e.bindTexture(k,M.__webglTexture,n.TEXTURE0+R);let nt=i.get(z);if(z.version!==nt.__version||K===!0){e.activeTexture(n.TEXTURE0+R);let Q=Jt.getPrimaries(Jt.workingColorSpace),X=v.colorSpace===qn?null:Jt.getPrimaries(v.colorSpace),ht=v.colorSpace===qn||Q===X?n.NONE:n.BROWSER_DEFAULT_WEBGL;n.pixelStorei(n.UNPACK_FLIP_Y_WEBGL,v.flipY),n.pixelStorei(n.UNPACK_PREMULTIPLY_ALPHA_WEBGL,v.premultiplyAlpha),n.pixelStorei(n.UNPACK_ALIGNMENT,v.unpackAlignment),n.pixelStorei(n.UNPACK_COLORSPACE_CONVERSION_WEBGL,ht);let tt=y(v.image,!1,s.maxTextureSize);tt=Rt(v,tt);let ct=r.convert(v.format,v.colorSpace),At=r.convert(v.type),rt=w(v.internalFormat,ct,At,v.colorSpace,v.isVideoTexture);St(k,v);let ot,Lt=v.mipmaps,wt=v.isVideoTexture!==!0,Ft=nt.__version===void 0||K===!0,L=z.dataReady,ft=A(v,tt);if(v.isDepthTexture)rt=_(v.format===ds,v.type),Ft&&(wt?e.texStorage2D(n.TEXTURE_2D,1,rt,tt.width,tt.height):e.texImage2D(n.TEXTURE_2D,0,rt,tt.width,tt.height,0,ct,At,null));else if(v.isDataTexture)if(Lt.length>0){wt&&Ft&&e.texStorage2D(n.TEXTURE_2D,ft,rt,Lt[0].width,Lt[0].height);for(let Y=0,$=Lt.length;Y<$;Y++)ot=Lt[Y],wt?L&&e.texSubImage2D(n.TEXTURE_2D,Y,0,0,ot.width,ot.height,ct,At,ot.data):e.texImage2D(n.TEXTURE_2D,Y,rt,ot.width,ot.height,0,ct,At,ot.data);v.generateMipmaps=!1}else wt?(Ft&&e.texStorage2D(n.TEXTURE_2D,ft,rt,tt.width,tt.height),L&&e.texSubImage2D(n.TEXTURE_2D,0,0,0,tt.width,tt.height,ct,At,tt.data)):e.texImage2D(n.TEXTURE_2D,0,rt,tt.width,tt.height,0,ct,At,tt.data);else if(v.isCompressedTexture)if(v.isCompressedArrayTexture){wt&&Ft&&e.texStorage3D(n.TEXTURE_2D_ARRAY,ft,rt,Lt[0].width,Lt[0].height,tt.depth);for(let Y=0,$=Lt.length;Y<$;Y++)if(ot=Lt[Y],v.format!==hn)if(ct!==null)if(wt){if(L)if(v.layerUpdates.size>0){let pt=tu(ot.width,ot.height,v.format,v.type);for(let vt of v.layerUpdates){let Xt=ot.data.subarray(vt*pt/ot.data.BYTES_PER_ELEMENT,(vt+1)*pt/ot.data.BYTES_PER_ELEMENT);e.compressedTexSubImage3D(n.TEXTURE_2D_ARRAY,Y,0,0,vt,ot.width,ot.height,1,ct,Xt,0,0)}v.clearLayerUpdates()}else e.compressedTexSubImage3D(n.TEXTURE_2D_ARRAY,Y,0,0,0,ot.width,ot.height,tt.depth,ct,ot.data,0,0)}else e.compressedTexImage3D(n.TEXTURE_2D_ARRAY,Y,rt,ot.width,ot.height,tt.depth,0,ot.data,0,0);else console.warn("THREE.WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()");else wt?L&&e.texSubImage3D(n.TEXTURE_2D_ARRAY,Y,0,0,0,ot.width,ot.height,tt.depth,ct,At,ot.data):e.texImage3D(n.TEXTURE_2D_ARRAY,Y,rt,ot.width,ot.height,tt.depth,0,ct,At,ot.data)}else{wt&&Ft&&e.texStorage2D(n.TEXTURE_2D,ft,rt,Lt[0].width,Lt[0].height);for(let Y=0,$=Lt.length;Y<$;Y++)ot=Lt[Y],v.format!==hn?ct!==null?wt?L&&e.compressedTexSubImage2D(n.TEXTURE_2D,Y,0,0,ot.width,ot.height,ct,ot.data):e.compressedTexImage2D(n.TEXTURE_2D,Y,rt,ot.width,ot.height,0,ot.data):console.warn("THREE.WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()"):wt?L&&e.texSubImage2D(n.TEXTURE_2D,Y,0,0,ot.width,ot.height,ct,At,ot.data):e.texImage2D(n.TEXTURE_2D,Y,rt,ot.width,ot.height,0,ct,At,ot.data)}else if(v.isDataArrayTexture)if(wt){if(Ft&&e.texStorage3D(n.TEXTURE_2D_ARRAY,ft,rt,tt.width,tt.height,tt.depth),L)if(v.layerUpdates.size>0){let Y=tu(tt.width,tt.height,v.format,v.type);for(let $ of v.layerUpdates){let pt=tt.data.subarray($*Y/tt.data.BYTES_PER_ELEMENT,($+1)*Y/tt.data.BYTES_PER_ELEMENT);e.texSubImage3D(n.TEXTURE_2D_ARRAY,0,0,0,$,tt.width,tt.height,1,ct,At,pt)}v.clearLayerUpdates()}else e.texSubImage3D(n.TEXTURE_2D_ARRAY,0,0,0,0,tt.width,tt.height,tt.depth,ct,At,tt.data)}else e.texImage3D(n.TEXTURE_2D_ARRAY,0,rt,tt.width,tt.height,tt.depth,0,ct,At,tt.data);else if(v.isData3DTexture)wt?(Ft&&e.texStorage3D(n.TEXTURE_3D,ft,rt,tt.width,tt.height,tt.depth),L&&e.texSubImage3D(n.TEXTURE_3D,0,0,0,0,tt.width,tt.height,tt.depth,ct,At,tt.data)):e.texImage3D(n.TEXTURE_3D,0,rt,tt.width,tt.height,tt.depth,0,ct,At,tt.data);else if(v.isFramebufferTexture){if(Ft)if(wt)e.texStorage2D(n.TEXTURE_2D,ft,rt,tt.width,tt.height);else{let Y=tt.width,$=tt.height;for(let pt=0;pt<ft;pt++)e.texImage2D(n.TEXTURE_2D,pt,rt,Y,$,0,ct,At,null),Y>>=1,$>>=1}}else if(Lt.length>0){if(wt&&Ft){let Y=B(Lt[0]);e.texStorage2D(n.TEXTURE_2D,ft,rt,Y.width,Y.height)}for(let Y=0,$=Lt.length;Y<$;Y++)ot=Lt[Y],wt?L&&e.texSubImage2D(n.TEXTURE_2D,Y,0,0,ct,At,ot):e.texImage2D(n.TEXTURE_2D,Y,rt,ct,At,ot);v.generateMipmaps=!1}else if(wt){if(Ft){let Y=B(tt);e.texStorage2D(n.TEXTURE_2D,ft,rt,Y.width,Y.height)}L&&e.texSubImage2D(n.TEXTURE_2D,0,0,0,ct,At,tt)}else e.texImage2D(n.TEXTURE_2D,0,rt,ct,At,tt);p(v)&&m(k),nt.__version=z.version,v.onUpdate&&v.onUpdate(v)}M.__version=v.version}function J(M,v,R){if(v.image.length!==6)return;let k=Gt(M,v),K=v.source;e.bindTexture(n.TEXTURE_CUBE_MAP,M.__webglTexture,n.TEXTURE0+R);let z=i.get(K);if(K.version!==z.__version||k===!0){e.activeTexture(n.TEXTURE0+R);let nt=Jt.getPrimaries(Jt.workingColorSpace),Q=v.colorSpace===qn?null:Jt.getPrimaries(v.colorSpace),X=v.colorSpace===qn||nt===Q?n.NONE:n.BROWSER_DEFAULT_WEBGL;n.pixelStorei(n.UNPACK_FLIP_Y_WEBGL,v.flipY),n.pixelStorei(n.UNPACK_PREMULTIPLY_ALPHA_WEBGL,v.premultiplyAlpha),n.pixelStorei(n.UNPACK_ALIGNMENT,v.unpackAlignment),n.pixelStorei(n.UNPACK_COLORSPACE_CONVERSION_WEBGL,X);let ht=v.isCompressedTexture||v.image[0].isCompressedTexture,tt=v.image[0]&&v.image[0].isDataTexture,ct=[];for(let $=0;$<6;$++)!ht&&!tt?ct[$]=y(v.image[$],!0,s.maxCubemapSize):ct[$]=tt?v.image[$].image:v.image[$],ct[$]=Rt(v,ct[$]);let At=ct[0],rt=r.convert(v.format,v.colorSpace),ot=r.convert(v.type),Lt=w(v.internalFormat,rt,ot,v.colorSpace),wt=v.isVideoTexture!==!0,Ft=z.__version===void 0||k===!0,L=K.dataReady,ft=A(v,At);St(n.TEXTURE_CUBE_MAP,v);let Y;if(ht){wt&&Ft&&e.texStorage2D(n.TEXTURE_CUBE_MAP,ft,Lt,At.width,At.height);for(let $=0;$<6;$++){Y=ct[$].mipmaps;for(let pt=0;pt<Y.length;pt++){let vt=Y[pt];v.format!==hn?rt!==null?wt?L&&e.compressedTexSubImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt,0,0,vt.width,vt.height,rt,vt.data):e.compressedTexImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt,Lt,vt.width,vt.height,0,vt.data):console.warn("THREE.WebGLRenderer: Attempt to load unsupported compressed texture format in .setTextureCube()"):wt?L&&e.texSubImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt,0,0,vt.width,vt.height,rt,ot,vt.data):e.texImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt,Lt,vt.width,vt.height,0,rt,ot,vt.data)}}}else{if(Y=v.mipmaps,wt&&Ft){Y.length>0&&ft++;let $=B(ct[0]);e.texStorage2D(n.TEXTURE_CUBE_MAP,ft,Lt,$.width,$.height)}for(let $=0;$<6;$++)if(tt){wt?L&&e.texSubImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,0,0,0,ct[$].width,ct[$].height,rt,ot,ct[$].data):e.texImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,0,Lt,ct[$].width,ct[$].height,0,rt,ot,ct[$].data);for(let pt=0;pt<Y.length;pt++){let Xt=Y[pt].image[$].image;wt?L&&e.texSubImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt+1,0,0,Xt.width,Xt.height,rt,ot,Xt.data):e.texImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt+1,Lt,Xt.width,Xt.height,0,rt,ot,Xt.data)}}else{wt?L&&e.texSubImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,0,0,0,rt,ot,ct[$]):e.texImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,0,Lt,rt,ot,ct[$]);for(let pt=0;pt<Y.length;pt++){let vt=Y[pt];wt?L&&e.texSubImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt+1,0,0,rt,ot,vt.image[$]):e.texImage2D(n.TEXTURE_CUBE_MAP_POSITIVE_X+$,pt+1,Lt,rt,ot,vt.image[$])}}}p(v)&&m(n.TEXTURE_CUBE_MAP),z.__version=K.version,v.onUpdate&&v.onUpdate(v)}M.__version=v.version}function st(M,v,R,k,K,z){let nt=r.convert(R.format,R.colorSpace),Q=r.convert(R.type),X=w(R.internalFormat,nt,Q,R.colorSpace);if(!i.get(v).__hasExternalTextures){let tt=Math.max(1,v.width>>z),ct=Math.max(1,v.height>>z);K===n.TEXTURE_3D||K===n.TEXTURE_2D_ARRAY?e.texImage3D(K,z,X,tt,ct,v.depth,0,nt,Q,null):e.texImage2D(K,z,X,tt,ct,0,nt,Q,null)}e.bindFramebuffer(n.FRAMEBUFFER,M),lt(v)?a.framebufferTexture2DMultisampleEXT(n.FRAMEBUFFER,k,K,i.get(R).__webglTexture,0,j(v)):(K===n.TEXTURE_2D||K>=n.TEXTURE_CUBE_MAP_POSITIVE_X&&K<=n.TEXTURE_CUBE_MAP_NEGATIVE_Z)&&n.framebufferTexture2D(n.FRAMEBUFFER,k,K,i.get(R).__webglTexture,z),e.bindFramebuffer(n.FRAMEBUFFER,null)}function Mt(M,v,R){if(n.bindRenderbuffer(n.RENDERBUFFER,M),v.depthBuffer){let k=v.depthTexture,K=k&&k.isDepthTexture?k.type:null,z=_(v.stencilBuffer,K),nt=v.stencilBuffer?n.DEPTH_STENCIL_ATTACHMENT:n.DEPTH_ATTACHMENT,Q=j(v);lt(v)?a.renderbufferStorageMultisampleEXT(n.RENDERBUFFER,Q,z,v.width,v.height):R?n.renderbufferStorageMultisample(n.RENDERBUFFER,Q,z,v.width,v.height):n.renderbufferStorage(n.RENDERBUFFER,z,v.width,v.height),n.framebufferRenderbuffer(n.FRAMEBUFFER,nt,n.RENDERBUFFER,M)}else{let k=v.textures;for(let K=0;K<k.length;K++){let z=k[K],nt=r.convert(z.format,z.colorSpace),Q=r.convert(z.type),X=w(z.internalFormat,nt,Q,z.colorSpace),ht=j(v);R&&lt(v)===!1?n.renderbufferStorageMultisample(n.RENDERBUFFER,ht,X,v.width,v.height):lt(v)?a.renderbufferStorageMultisampleEXT(n.RENDERBUFFER,ht,X,v.width,v.height):n.renderbufferStorage(n.RENDERBUFFER,X,v.width,v.height)}}n.bindRenderbuffer(n.RENDERBUFFER,null)}function xt(M,v){if(v&&v.isWebGLCubeRenderTarget)throw new Error("Depth Texture with cube render targets is not supported");if(e.bindFramebuffer(n.FRAMEBUFFER,M),!(v.depthTexture&&v.depthTexture.isDepthTexture))throw new Error("renderTarget.depthTexture must be an instance of THREE.DepthTexture");(!i.get(v.depthTexture).__webglTexture||v.depthTexture.image.width!==v.width||v.depthTexture.image.height!==v.height)&&(v.depthTexture.image.width=v.width,v.depthTexture.image.height=v.height,v.depthTexture.needsUpdate=!0),q(v.depthTexture,0);let k=i.get(v.depthTexture).__webglTexture,K=j(v);if(v.depthTexture.format===ss)lt(v)?a.framebufferTexture2DMultisampleEXT(n.FRAMEBUFFER,n.DEPTH_ATTACHMENT,n.TEXTURE_2D,k,0,K):n.framebufferTexture2D(n.FRAMEBUFFER,n.DEPTH_ATTACHMENT,n.TEXTURE_2D,k,0);else if(v.depthTexture.format===ds)lt(v)?a.framebufferTexture2DMultisampleEXT(n.FRAMEBUFFER,n.DEPTH_STENCIL_ATTACHMENT,n.TEXTURE_2D,k,0,K):n.framebufferTexture2D(n.FRAMEBUFFER,n.DEPTH_STENCIL_ATTACHMENT,n.TEXTURE_2D,k,0);else throw new Error("Unknown depthTexture format")}function Nt(M){let v=i.get(M),R=M.isWebGLCubeRenderTarget===!0;if(v.__boundDepthTexture!==M.depthTexture){let k=M.depthTexture;if(v.__depthDisposeCallback&&v.__depthDisposeCallback(),k){let K=()=>{delete v.__boundDepthTexture,delete v.__depthDisposeCallback,k.removeEventListener("dispose",K)};k.addEventListener("dispose",K),v.__depthDisposeCallback=K}v.__boundDepthTexture=k}if(M.depthTexture&&!v.__autoAllocateDepthBuffer){if(R)throw new Error("target.depthTexture not supported in Cube render targets");xt(v.__webglFramebuffer,M)}else if(R){v.__webglDepthbuffer=[];for(let k=0;k<6;k++)if(e.bindFramebuffer(n.FRAMEBUFFER,v.__webglFramebuffer[k]),v.__webglDepthbuffer[k]===void 0)v.__webglDepthbuffer[k]=n.createRenderbuffer(),Mt(v.__webglDepthbuffer[k],M,!1);else{let K=M.stencilBuffer?n.DEPTH_STENCIL_ATTACHMENT:n.DEPTH_ATTACHMENT,z=v.__webglDepthbuffer[k];n.bindRenderbuffer(n.RENDERBUFFER,z),n.framebufferRenderbuffer(n.FRAMEBUFFER,K,n.RENDERBUFFER,z)}}else if(e.bindFramebuffer(n.FRAMEBUFFER,v.__webglFramebuffer),v.__webglDepthbuffer===void 0)v.__webglDepthbuffer=n.createRenderbuffer(),Mt(v.__webglDepthbuffer,M,!1);else{let k=M.stencilBuffer?n.DEPTH_STENCIL_ATTACHMENT:n.DEPTH_ATTACHMENT,K=v.__webglDepthbuffer;n.bindRenderbuffer(n.RENDERBUFFER,K),n.framebufferRenderbuffer(n.FRAMEBUFFER,k,n.RENDERBUFFER,K)}e.bindFramebuffer(n.FRAMEBUFFER,null)}function It(M,v,R){let k=i.get(M);v!==void 0&&st(k.__webglFramebuffer,M,M.texture,n.COLOR_ATTACHMENT0,n.TEXTURE_2D,0),R!==void 0&&Nt(M)}function kt(M){let v=M.texture,R=i.get(M),k=i.get(v);M.addEventListener("dispose",C);let K=M.textures,z=M.isWebGLCubeRenderTarget===!0,nt=K.length>1;if(nt||(k.__webglTexture===void 0&&(k.__webglTexture=n.createTexture()),k.__version=v.version,o.memory.textures++),z){R.__webglFramebuffer=[];for(let Q=0;Q<6;Q++)if(v.mipmaps&&v.mipmaps.length>0){R.__webglFramebuffer[Q]=[];for(let X=0;X<v.mipmaps.length;X++)R.__webglFramebuffer[Q][X]=n.createFramebuffer()}else R.__webglFramebuffer[Q]=n.createFramebuffer()}else{if(v.mipmaps&&v.mipmaps.length>0){R.__webglFramebuffer=[];for(let Q=0;Q<v.mipmaps.length;Q++)R.__webglFramebuffer[Q]=n.createFramebuffer()}else R.__webglFramebuffer=n.createFramebuffer();if(nt)for(let Q=0,X=K.length;Q<X;Q++){let ht=i.get(K[Q]);ht.__webglTexture===void 0&&(ht.__webglTexture=n.createTexture(),o.memory.textures++)}if(M.samples>0&&lt(M)===!1){R.__webglMultisampledFramebuffer=n.createFramebuffer(),R.__webglColorRenderbuffer=[],e.bindFramebuffer(n.FRAMEBUFFER,R.__webglMultisampledFramebuffer);for(let Q=0;Q<K.length;Q++){let X=K[Q];R.__webglColorRenderbuffer[Q]=n.createRenderbuffer(),n.bindRenderbuffer(n.RENDERBUFFER,R.__webglColorRenderbuffer[Q]);let ht=r.convert(X.format,X.colorSpace),tt=r.convert(X.type),ct=w(X.internalFormat,ht,tt,X.colorSpace,M.isXRRenderTarget===!0),At=j(M);n.renderbufferStorageMultisample(n.RENDERBUFFER,At,ct,M.width,M.height),n.framebufferRenderbuffer(n.FRAMEBUFFER,n.COLOR_ATTACHMENT0+Q,n.RENDERBUFFER,R.__webglColorRenderbuffer[Q])}n.bindRenderbuffer(n.RENDERBUFFER,null),M.depthBuffer&&(R.__webglDepthRenderbuffer=n.createRenderbuffer(),Mt(R.__webglDepthRenderbuffer,M,!0)),e.bindFramebuffer(n.FRAMEBUFFER,null)}}if(z){e.bindTexture(n.TEXTURE_CUBE_MAP,k.__webglTexture),St(n.TEXTURE_CUBE_MAP,v);for(let Q=0;Q<6;Q++)if(v.mipmaps&&v.mipmaps.length>0)for(let X=0;X<v.mipmaps.length;X++)st(R.__webglFramebuffer[Q][X],M,v,n.COLOR_ATTACHMENT0,n.TEXTURE_CUBE_MAP_POSITIVE_X+Q,X);else st(R.__webglFramebuffer[Q],M,v,n.COLOR_ATTACHMENT0,n.TEXTURE_CUBE_MAP_POSITIVE_X+Q,0);p(v)&&m(n.TEXTURE_CUBE_MAP),e.unbindTexture()}else if(nt){for(let Q=0,X=K.length;Q<X;Q++){let ht=K[Q],tt=i.get(ht);e.bindTexture(n.TEXTURE_2D,tt.__webglTexture),St(n.TEXTURE_2D,ht),st(R.__webglFramebuffer,M,ht,n.COLOR_ATTACHMENT0+Q,n.TEXTURE_2D,0),p(ht)&&m(n.TEXTURE_2D)}e.unbindTexture()}else{let Q=n.TEXTURE_2D;if((M.isWebGL3DRenderTarget||M.isWebGLArrayRenderTarget)&&(Q=M.isWebGL3DRenderTarget?n.TEXTURE_3D:n.TEXTURE_2D_ARRAY),e.bindTexture(Q,k.__webglTexture),St(Q,v),v.mipmaps&&v.mipmaps.length>0)for(let X=0;X<v.mipmaps.length;X++)st(R.__webglFramebuffer[X],M,v,n.COLOR_ATTACHMENT0,Q,X);else st(R.__webglFramebuffer,M,v,n.COLOR_ATTACHMENT0,Q,0);p(v)&&m(Q),e.unbindTexture()}M.depthBuffer&&Nt(M)}function Zt(M){let v=M.textures;for(let R=0,k=v.length;R<k;R++){let K=v[R];if(p(K)){let z=M.isWebGLCubeRenderTarget?n.TEXTURE_CUBE_MAP:n.TEXTURE_2D,nt=i.get(K).__webglTexture;e.bindTexture(z,nt),m(z),e.unbindTexture()}}}let Vt=[],P=[];function fe(M){if(M.samples>0){if(lt(M)===!1){let v=M.textures,R=M.width,k=M.height,K=n.COLOR_BUFFER_BIT,z=M.stencilBuffer?n.DEPTH_STENCIL_ATTACHMENT:n.DEPTH_ATTACHMENT,nt=i.get(M),Q=v.length>1;if(Q)for(let X=0;X<v.length;X++)e.bindFramebuffer(n.FRAMEBUFFER,nt.__webglMultisampledFramebuffer),n.framebufferRenderbuffer(n.FRAMEBUFFER,n.COLOR_ATTACHMENT0+X,n.RENDERBUFFER,null),e.bindFramebuffer(n.FRAMEBUFFER,nt.__webglFramebuffer),n.framebufferTexture2D(n.DRAW_FRAMEBUFFER,n.COLOR_ATTACHMENT0+X,n.TEXTURE_2D,null,0);e.bindFramebuffer(n.READ_FRAMEBUFFER,nt.__webglMultisampledFramebuffer),e.bindFramebuffer(n.DRAW_FRAMEBUFFER,nt.__webglFramebuffer);for(let X=0;X<v.length;X++){if(M.resolveDepthBuffer&&(M.depthBuffer&&(K|=n.DEPTH_BUFFER_BIT),M.stencilBuffer&&M.resolveStencilBuffer&&(K|=n.STENCIL_BUFFER_BIT)),Q){n.framebufferRenderbuffer(n.READ_FRAMEBUFFER,n.COLOR_ATTACHMENT0,n.RENDERBUFFER,nt.__webglColorRenderbuffer[X]);let ht=i.get(v[X]).__webglTexture;n.framebufferTexture2D(n.DRAW_FRAMEBUFFER,n.COLOR_ATTACHMENT0,n.TEXTURE_2D,ht,0)}n.blitFramebuffer(0,0,R,k,0,0,R,k,K,n.NEAREST),l===!0&&(Vt.length=0,P.length=0,Vt.push(n.COLOR_ATTACHMENT0+X),M.depthBuffer&&M.resolveDepthBuffer===!1&&(Vt.push(z),P.push(z),n.invalidateFramebuffer(n.DRAW_FRAMEBUFFER,P)),n.invalidateFramebuffer(n.READ_FRAMEBUFFER,Vt))}if(e.bindFramebuffer(n.READ_FRAMEBUFFER,null),e.bindFramebuffer(n.DRAW_FRAMEBUFFER,null),Q)for(let X=0;X<v.length;X++){e.bindFramebuffer(n.FRAMEBUFFER,nt.__webglMultisampledFramebuffer),n.framebufferRenderbuffer(n.FRAMEBUFFER,n.COLOR_ATTACHMENT0+X,n.RENDERBUFFER,nt.__webglColorRenderbuffer[X]);let ht=i.get(v[X]).__webglTexture;e.bindFramebuffer(n.FRAMEBUFFER,nt.__webglFramebuffer),n.framebufferTexture2D(n.DRAW_FRAMEBUFFER,n.COLOR_ATTACHMENT0+X,n.TEXTURE_2D,ht,0)}e.bindFramebuffer(n.DRAW_FRAMEBUFFER,nt.__webglMultisampledFramebuffer)}else if(M.depthBuffer&&M.resolveDepthBuffer===!1&&l){let v=M.stencilBuffer?n.DEPTH_STENCIL_ATTACHMENT:n.DEPTH_ATTACHMENT;n.invalidateFramebuffer(n.DRAW_FRAMEBUFFER,[v])}}}function j(M){return Math.min(s.maxSamples,M.samples)}function lt(M){let v=i.get(M);return M.samples>0&&t.has("WEBGL_multisampled_render_to_texture")===!0&&v.__useRenderToTexture!==!1}function ut(M){let v=o.render.frame;f.get(M)!==v&&(f.set(M,v),M.update())}function Rt(M,v){let R=M.colorSpace,k=M.format,K=M.type;return M.isCompressedTexture===!0||M.isVideoTexture===!0||R!==Kn&&R!==qn&&(Jt.getTransfer(R)===ee?(k!==hn||K!==In)&&console.warn("THREE.WebGLTextures: sRGB encoded textures have to use RGBAFormat and UnsignedByteType."):console.error("THREE.WebGLTextures: Unsupported texture color space:",R)),v}function B(M){return typeof HTMLImageElement<"u"&&M instanceof HTMLImageElement?(h.width=M.naturalWidth||M.width,h.height=M.naturalHeight||M.height):typeof VideoFrame<"u"&&M instanceof VideoFrame?(h.width=M.displayWidth,h.height=M.displayHeight):(h.width=M.width,h.height=M.height),h}this.allocateTextureUnit=O,this.resetTextureUnits=S,this.setTexture2D=q,this.setTexture2DArray=et,this.setTexture3D=F,this.setTextureCube=it,this.rebindTextures=It,this.setupRenderTarget=kt,this.updateRenderTargetMipmap=Zt,this.updateMultisampleRenderTarget=fe,this.setupDepthRenderbuffer=Nt,this.setupFrameBufferTexture=st,this.useMultisampledRTT=lt}function Cx(n,t){function e(i,s=qn){let r,o=Jt.getTransfer(s);if(i===In)return n.UNSIGNED_BYTE;if(i===oc)return n.UNSIGNED_SHORT_4_4_4_4;if(i===ac)return n.UNSIGNED_SHORT_5_5_5_1;if(i===fu)return n.UNSIGNED_INT_5_9_9_9_REV;if(i===uu)return n.BYTE;if(i===du)return n.SHORT;if(i===Qs)return n.UNSIGNED_SHORT;if(i===rc)return n.INT;if(i===vi)return n.UNSIGNED_INT;if(i===xn)return n.FLOAT;if(i===ze)return n.HALF_FLOAT;if(i===pu)return n.ALPHA;if(i===mu)return n.RGB;if(i===hn)return n.RGBA;if(i===gu)return n.LUMINANCE;if(i===xu)return n.LUMINANCE_ALPHA;if(i===ss)return n.DEPTH_COMPONENT;if(i===ds)return n.DEPTH_STENCIL;if(i===lc)return n.RED;if(i===cc)return n.RED_INTEGER;if(i===vu)return n.RG;if(i===hc)return n.RG_INTEGER;if(i===uc)return n.RGBA_INTEGER;if(i===Gr||i===Wr||i===Xr||i===qr)if(o===ee)if(r=t.get("WEBGL_compressed_texture_s3tc_srgb"),r!==null){if(i===Gr)return r.COMPRESSED_SRGB_S3TC_DXT1_EXT;if(i===Wr)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT1_EXT;if(i===Xr)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT3_EXT;if(i===qr)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT5_EXT}else return null;else if(r=t.get("WEBGL_compressed_texture_s3tc"),r!==null){if(i===Gr)return r.COMPRESSED_RGB_S3TC_DXT1_EXT;if(i===Wr)return r.COMPRESSED_RGBA_S3TC_DXT1_EXT;if(i===Xr)return r.COMPRESSED_RGBA_S3TC_DXT3_EXT;if(i===qr)return r.COMPRESSED_RGBA_S3TC_DXT5_EXT}else return null;if(i===el||i===nl||i===il||i===sl)if(r=t.get("WEBGL_compressed_texture_pvrtc"),r!==null){if(i===el)return r.COMPRESSED_RGB_PVRTC_4BPPV1_IMG;if(i===nl)return r.COMPRESSED_RGB_PVRTC_2BPPV1_IMG;if(i===il)return r.COMPRESSED_RGBA_PVRTC_4BPPV1_IMG;if(i===sl)return r.COMPRESSED_RGBA_PVRTC_2BPPV1_IMG}else return null;if(i===rl||i===ol||i===al)if(r=t.get("WEBGL_compressed_texture_etc"),r!==null){if(i===rl||i===ol)return o===ee?r.COMPRESSED_SRGB8_ETC2:r.COMPRESSED_RGB8_ETC2;if(i===al)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ETC2_EAC:r.COMPRESSED_RGBA8_ETC2_EAC}else return null;if(i===ll||i===cl||i===hl||i===ul||i===dl||i===fl||i===pl||i===ml||i===gl||i===xl||i===vl||i===_l||i===yl||i===Ml)if(r=t.get("WEBGL_compressed_texture_astc"),r!==null){if(i===ll)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_4x4_KHR:r.COMPRESSED_RGBA_ASTC_4x4_KHR;if(i===cl)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x4_KHR:r.COMPRESSED_RGBA_ASTC_5x4_KHR;if(i===hl)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x5_KHR:r.COMPRESSED_RGBA_ASTC_5x5_KHR;if(i===ul)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x5_KHR:r.COMPRESSED_RGBA_ASTC_6x5_KHR;if(i===dl)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x6_KHR:r.COMPRESSED_RGBA_ASTC_6x6_KHR;if(i===fl)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x5_KHR:r.COMPRESSED_RGBA_ASTC_8x5_KHR;if(i===pl)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x6_KHR:r.COMPRESSED_RGBA_ASTC_8x6_KHR;if(i===ml)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x8_KHR:r.COMPRESSED_RGBA_ASTC_8x8_KHR;if(i===gl)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x5_KHR:r.COMPRESSED_RGBA_ASTC_10x5_KHR;if(i===xl)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x6_KHR:r.COMPRESSED_RGBA_ASTC_10x6_KHR;if(i===vl)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x8_KHR:r.COMPRESSED_RGBA_ASTC_10x8_KHR;if(i===_l)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x10_KHR:r.COMPRESSED_RGBA_ASTC_10x10_KHR;if(i===yl)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x10_KHR:r.COMPRESSED_RGBA_ASTC_12x10_KHR;if(i===Ml)return o===ee?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x12_KHR:r.COMPRESSED_RGBA_ASTC_12x12_KHR}else return null;if(i===Yr||i===Sl||i===bl)if(r=t.get("EXT_texture_compression_bptc"),r!==null){if(i===Yr)return o===ee?r.COMPRESSED_SRGB_ALPHA_BPTC_UNORM_EXT:r.COMPRESSED_RGBA_BPTC_UNORM_EXT;if(i===Sl)return r.COMPRESSED_RGB_BPTC_SIGNED_FLOAT_EXT;if(i===bl)return r.COMPRESSED_RGB_BPTC_UNSIGNED_FLOAT_EXT}else return null;if(i===_u||i===wl||i===Al||i===El)if(r=t.get("EXT_texture_compression_rgtc"),r!==null){if(i===Yr)return r.COMPRESSED_RED_RGTC1_EXT;if(i===wl)return r.COMPRESSED_SIGNED_RED_RGTC1_EXT;if(i===Al)return r.COMPRESSED_RED_GREEN_RGTC2_EXT;if(i===El)return r.COMPRESSED_SIGNED_RED_GREEN_RGTC2_EXT}else return null;return i===us?n.UNSIGNED_INT_24_8:n[i]!==void 0?n[i]:null}return{convert:e}}var Hl=class extends De{constructor(t=[]){super(),this.isArrayCamera=!0,this.cameras=t}},Yn=class extends Be{constructor(){super(),this.isGroup=!0,this.type="Group"}},Rx={type:"move"},Js=class{constructor(){this._targetRay=null,this._grip=null,this._hand=null}getHandSpace(){return this._hand===null&&(this._hand=new Yn,this._hand.matrixAutoUpdate=!1,this._hand.visible=!1,this._hand.joints={},this._hand.inputState={pinching:!1}),this._hand}getTargetRaySpace(){return this._targetRay===null&&(this._targetRay=new Yn,this._targetRay.matrixAutoUpdate=!1,this._targetRay.visible=!1,this._targetRay.hasLinearVelocity=!1,this._targetRay.linearVelocity=new U,this._targetRay.hasAngularVelocity=!1,this._targetRay.angularVelocity=new U),this._targetRay}getGripSpace(){return this._grip===null&&(this._grip=new Yn,this._grip.matrixAutoUpdate=!1,this._grip.visible=!1,this._grip.hasLinearVelocity=!1,this._grip.linearVelocity=new U,this._grip.hasAngularVelocity=!1,this._grip.angularVelocity=new U),this._grip}dispatchEvent(t){return this._targetRay!==null&&this._targetRay.dispatchEvent(t),this._grip!==null&&this._grip.dispatchEvent(t),this._hand!==null&&this._hand.dispatchEvent(t),this}connect(t){if(t&&t.hand){let e=this._hand;if(e)for(let i of t.hand.values())this._getHandJoint(e,i)}return this.dispatchEvent({type:"connected",data:t}),this}disconnect(t){return this.dispatchEvent({type:"disconnected",data:t}),this._targetRay!==null&&(this._targetRay.visible=!1),this._grip!==null&&(this._grip.visible=!1),this._hand!==null&&(this._hand.visible=!1),this}update(t,e,i){let s=null,r=null,o=null,a=this._targetRay,l=this._grip,h=this._hand;if(t&&e.session.visibilityState!=="visible-blurred"){if(h&&t.hand){o=!0;for(let y of t.hand.values()){let p=e.getJointPose(y,i),m=this._getHandJoint(h,y);p!==null&&(m.matrix.fromArray(p.transform.matrix),m.matrix.decompose(m.position,m.rotation,m.scale),m.matrixWorldNeedsUpdate=!0,m.jointRadius=p.radius),m.visible=p!==null}let f=h.joints["index-finger-tip"],d=h.joints["thumb-tip"],c=f.position.distanceTo(d.position),u=.02,g=.005;h.inputState.pinching&&c>u+g?(h.inputState.pinching=!1,this.dispatchEvent({type:"pinchend",handedness:t.handedness,target:this})):!h.inputState.pinching&&c<=u-g&&(h.inputState.pinching=!0,this.dispatchEvent({type:"pinchstart",handedness:t.handedness,target:this}))}else l!==null&&t.gripSpace&&(r=e.getPose(t.gripSpace,i),r!==null&&(l.matrix.fromArray(r.transform.matrix),l.matrix.decompose(l.position,l.rotation,l.scale),l.matrixWorldNeedsUpdate=!0,r.linearVelocity?(l.hasLinearVelocity=!0,l.linearVelocity.copy(r.linearVelocity)):l.hasLinearVelocity=!1,r.angularVelocity?(l.hasAngularVelocity=!0,l.angularVelocity.copy(r.angularVelocity)):l.hasAngularVelocity=!1));a!==null&&(s=e.getPose(t.targetRaySpace,i),s===null&&r!==null&&(s=r),s!==null&&(a.matrix.fromArray(s.transform.matrix),a.matrix.decompose(a.position,a.rotation,a.scale),a.matrixWorldNeedsUpdate=!0,s.linearVelocity?(a.hasLinearVelocity=!0,a.linearVelocity.copy(s.linearVelocity)):a.hasLinearVelocity=!1,s.angularVelocity?(a.hasAngularVelocity=!0,a.angularVelocity.copy(s.angularVelocity)):a.hasAngularVelocity=!1,this.dispatchEvent(Rx)))}return a!==null&&(a.visible=s!==null),l!==null&&(l.visible=r!==null),h!==null&&(h.visible=o!==null),this}_getHandJoint(t,e){if(t.joints[e.jointName]===void 0){let i=new Yn;i.matrixAutoUpdate=!1,i.visible=!1,t.joints[e.jointName]=i,t.add(i)}return t.joints[e.jointName]}},Px=`
void main() {

	gl_Position = vec4( position, 1.0 );

}`,Ix=`
uniform sampler2DArray depthColor;
uniform float depthWidth;
uniform float depthHeight;

void main() {

	vec2 coord = vec2( gl_FragCoord.x / depthWidth, gl_FragCoord.y / depthHeight );

	if ( coord.x >= 1.0 ) {

		gl_FragDepth = texture( depthColor, vec3( coord.x - 1.0, coord.y, 1 ) ).r;

	} else {

		gl_FragDepth = texture( depthColor, vec3( coord.x, coord.y, 0 ) ).r;

	}

}`,Vl=class{constructor(){this.texture=null,this.mesh=null,this.depthNear=0,this.depthFar=0}init(t,e,i){if(this.texture===null){let s=new Ce,r=t.properties.get(s);r.__webglTexture=e.texture,(e.depthNear!=i.depthNear||e.depthFar!=i.depthFar)&&(this.depthNear=e.depthNear,this.depthFar=e.depthFar),this.texture=s}}getMesh(t){if(this.texture!==null&&this.mesh===null){let e=t.cameras[0].viewport,i=new ie({vertexShader:Px,fragmentShader:Ix,uniforms:{depthColor:{value:this.texture},depthWidth:{value:e.z},depthHeight:{value:e.w}}});this.mesh=new Ue(new lo(20,20),i)}return this.mesh}reset(){this.texture=null,this.mesh=null}getDepthTexture(){return this.texture}},Gl=class extends Ln{constructor(t,e){super();let i=this,s=null,r=1,o=null,a="local-floor",l=1,h=null,f=null,d=null,c=null,u=null,g=null,y=new Vl,p=e.getContextAttributes(),m=null,w=null,_=[],A=[],I=new yt,C=null,E=new De;E.layers.enable(1),E.viewport=new oe;let T=new De;T.layers.enable(2),T.viewport=new oe;let H=[E,T],x=new Hl;x.layers.enable(1),x.layers.enable(2);let S=null,O=null;this.cameraAutoUpdate=!0,this.enabled=!1,this.isPresenting=!1,this.getController=function(J){let st=_[J];return st===void 0&&(st=new Js,_[J]=st),st.getTargetRaySpace()},this.getControllerGrip=function(J){let st=_[J];return st===void 0&&(st=new Js,_[J]=st),st.getGripSpace()},this.getHand=function(J){let st=_[J];return st===void 0&&(st=new Js,_[J]=st),st.getHandSpace()};function W(J){let st=A.indexOf(J.inputSource);if(st===-1)return;let Mt=_[st];Mt!==void 0&&(Mt.update(J.inputSource,J.frame,h||o),Mt.dispatchEvent({type:J.type,data:J.inputSource}))}function q(){s.removeEventListener("select",W),s.removeEventListener("selectstart",W),s.removeEventListener("selectend",W),s.removeEventListener("squeeze",W),s.removeEventListener("squeezestart",W),s.removeEventListener("squeezeend",W),s.removeEventListener("end",q),s.removeEventListener("inputsourceschange",et);for(let J=0;J<_.length;J++){let st=A[J];st!==null&&(A[J]=null,_[J].disconnect(st))}S=null,O=null,y.reset(),t.setRenderTarget(m),u=null,c=null,d=null,s=null,w=null,Wt.stop(),i.isPresenting=!1,t.setPixelRatio(C),t.setSize(I.width,I.height,!1),i.dispatchEvent({type:"sessionend"})}this.setFramebufferScaleFactor=function(J){r=J,i.isPresenting===!0&&console.warn("THREE.WebXRManager: Cannot change framebuffer scale while presenting.")},this.setReferenceSpaceType=function(J){a=J,i.isPresenting===!0&&console.warn("THREE.WebXRManager: Cannot change reference space type while presenting.")},this.getReferenceSpace=function(){return h||o},this.setReferenceSpace=function(J){h=J},this.getBaseLayer=function(){return c!==null?c:u},this.getBinding=function(){return d},this.getFrame=function(){return g},this.getSession=function(){return s},this.setSession=async function(J){if(s=J,s!==null){if(m=t.getRenderTarget(),s.addEventListener("select",W),s.addEventListener("selectstart",W),s.addEventListener("selectend",W),s.addEventListener("squeeze",W),s.addEventListener("squeezestart",W),s.addEventListener("squeezeend",W),s.addEventListener("end",q),s.addEventListener("inputsourceschange",et),p.xrCompatible!==!0&&await e.makeXRCompatible(),C=t.getPixelRatio(),t.getSize(I),s.renderState.layers===void 0){let st={antialias:p.antialias,alpha:!0,depth:p.depth,stencil:p.stencil,framebufferScaleFactor:r};u=new XRWebGLLayer(s,e,st),s.updateRenderState({baseLayer:u}),t.setPixelRatio(1),t.setSize(u.framebufferWidth,u.framebufferHeight,!1),w=new ue(u.framebufferWidth,u.framebufferHeight,{format:hn,type:In,colorSpace:t.outputColorSpace,stencilBuffer:p.stencil})}else{let st=null,Mt=null,xt=null;p.depth&&(xt=p.stencil?e.DEPTH24_STENCIL8:e.DEPTH_COMPONENT24,st=p.stencil?ds:ss,Mt=p.stencil?us:vi);let Nt={colorFormat:e.RGBA8,depthFormat:xt,scaleFactor:r};d=new XRWebGLBinding(s,e),c=d.createProjectionLayer(Nt),s.updateRenderState({layers:[c]}),t.setPixelRatio(1),t.setSize(c.textureWidth,c.textureHeight,!1),w=new ue(c.textureWidth,c.textureHeight,{format:hn,type:In,depthTexture:new ho(c.textureWidth,c.textureHeight,Mt,void 0,void 0,void 0,void 0,void 0,void 0,st),stencilBuffer:p.stencil,colorSpace:t.outputColorSpace,samples:p.antialias?4:0,resolveDepthBuffer:c.ignoreDepthValues===!1})}w.isXRRenderTarget=!0,this.setFoveation(l),h=null,o=await s.requestReferenceSpace(a),Wt.setContext(s),Wt.start(),i.isPresenting=!0,i.dispatchEvent({type:"sessionstart"})}},this.getEnvironmentBlendMode=function(){if(s!==null)return s.environmentBlendMode},this.getDepthTexture=function(){return y.getDepthTexture()};function et(J){for(let st=0;st<J.removed.length;st++){let Mt=J.removed[st],xt=A.indexOf(Mt);xt>=0&&(A[xt]=null,_[xt].disconnect(Mt))}for(let st=0;st<J.added.length;st++){let Mt=J.added[st],xt=A.indexOf(Mt);if(xt===-1){for(let It=0;It<_.length;It++)if(It>=A.length){A.push(Mt),xt=It;break}else if(A[It]===null){A[It]=Mt,xt=It;break}if(xt===-1)break}let Nt=_[xt];Nt&&Nt.connect(Mt)}}let F=new U,it=new U;function Z(J,st,Mt){F.setFromMatrixPosition(st.matrixWorld),it.setFromMatrixPosition(Mt.matrixWorld);let xt=F.distanceTo(it),Nt=st.projectionMatrix.elements,It=Mt.projectionMatrix.elements,kt=Nt[14]/(Nt[10]-1),Zt=Nt[14]/(Nt[10]+1),Vt=(Nt[9]+1)/Nt[5],P=(Nt[9]-1)/Nt[5],fe=(Nt[8]-1)/Nt[0],j=(It[8]+1)/It[0],lt=kt*fe,ut=kt*j,Rt=xt/(-fe+j),B=Rt*-fe;if(st.matrixWorld.decompose(J.position,J.quaternion,J.scale),J.translateX(B),J.translateZ(Rt),J.matrixWorld.compose(J.position,J.quaternion,J.scale),J.matrixWorldInverse.copy(J.matrixWorld).invert(),Nt[10]===-1)J.projectionMatrix.copy(st.projectionMatrix),J.projectionMatrixInverse.copy(st.projectionMatrixInverse);else{let M=kt+Rt,v=Zt+Rt,R=lt-B,k=ut+(xt-B),K=Vt*Zt/v*M,z=P*Zt/v*M;J.projectionMatrix.makePerspective(R,k,K,z,M,v),J.projectionMatrixInverse.copy(J.projectionMatrix).invert()}}function gt(J,st){st===null?J.matrixWorld.copy(J.matrix):J.matrixWorld.multiplyMatrices(st.matrixWorld,J.matrix),J.matrixWorldInverse.copy(J.matrixWorld).invert()}this.updateCamera=function(J){if(s===null)return;let st=J.near,Mt=J.far;y.texture!==null&&(y.depthNear>0&&(st=y.depthNear),y.depthFar>0&&(Mt=y.depthFar)),x.near=T.near=E.near=st,x.far=T.far=E.far=Mt,(S!==x.near||O!==x.far)&&(s.updateRenderState({depthNear:x.near,depthFar:x.far}),S=x.near,O=x.far);let xt=J.parent,Nt=x.cameras;gt(x,xt);for(let It=0;It<Nt.length;It++)gt(Nt[It],xt);Nt.length===2?Z(x,E,T):x.projectionMatrix.copy(E.projectionMatrix),_t(J,x,xt)};function _t(J,st,Mt){Mt===null?J.matrix.copy(st.matrixWorld):(J.matrix.copy(Mt.matrixWorld),J.matrix.invert(),J.matrix.multiply(st.matrixWorld)),J.matrix.decompose(J.position,J.quaternion,J.scale),J.updateMatrixWorld(!0),J.projectionMatrix.copy(st.projectionMatrix),J.projectionMatrixInverse.copy(st.projectionMatrixInverse),J.isPerspectiveCamera&&(J.fov=$s*2*Math.atan(1/J.projectionMatrix.elements[5]),J.zoom=1)}this.getCamera=function(){return x},this.getFoveation=function(){if(!(c===null&&u===null))return l},this.setFoveation=function(J){l=J,c!==null&&(c.fixedFoveation=J),u!==null&&u.fixedFoveation!==void 0&&(u.fixedFoveation=J)},this.hasDepthSensing=function(){return y.texture!==null},this.getDepthSensingMesh=function(){return y.getMesh(x)};let St=null;function Gt(J,st){if(f=st.getViewerPose(h||o),g=st,f!==null){let Mt=f.views;u!==null&&(t.setRenderTargetFramebuffer(w,u.framebuffer),t.setRenderTarget(w));let xt=!1;Mt.length!==x.cameras.length&&(x.cameras.length=0,xt=!0);for(let It=0;It<Mt.length;It++){let kt=Mt[It],Zt=null;if(u!==null)Zt=u.getViewport(kt);else{let P=d.getViewSubImage(c,kt);Zt=P.viewport,It===0&&(t.setRenderTargetTextures(w,P.colorTexture,c.ignoreDepthValues?void 0:P.depthStencilTexture),t.setRenderTarget(w))}let Vt=H[It];Vt===void 0&&(Vt=new De,Vt.layers.enable(It),Vt.viewport=new oe,H[It]=Vt),Vt.matrix.fromArray(kt.transform.matrix),Vt.matrix.decompose(Vt.position,Vt.quaternion,Vt.scale),Vt.projectionMatrix.fromArray(kt.projectionMatrix),Vt.projectionMatrixInverse.copy(Vt.projectionMatrix).invert(),Vt.viewport.set(Zt.x,Zt.y,Zt.width,Zt.height),It===0&&(x.matrix.copy(Vt.matrix),x.matrix.decompose(x.position,x.quaternion,x.scale)),xt===!0&&x.cameras.push(Vt)}let Nt=s.enabledFeatures;if(Nt&&Nt.includes("depth-sensing")){let It=d.getDepthInformation(Mt[0]);It&&It.isValid&&It.texture&&y.init(t,It,s.renderState)}}for(let Mt=0;Mt<_.length;Mt++){let xt=A[Mt],Nt=_[Mt];xt!==null&&Nt!==void 0&&Nt.update(xt,st,h||o)}St&&St(J,st),st.detectedPlanes&&i.dispatchEvent({type:"planesdetected",data:st}),g=null}let Wt=new Au;Wt.setAnimationLoop(Gt),this.setAnimationLoop=function(J){St=J},this.dispose=function(){}}},ui=new tn,Lx=new $t;function Dx(n,t){function e(p,m){p.matrixAutoUpdate===!0&&p.updateMatrix(),m.value.copy(p.matrix)}function i(p,m){m.color.getRGB(p.fogColor.value,wu(n)),m.isFog?(p.fogNear.value=m.near,p.fogFar.value=m.far):m.isFogExp2&&(p.fogDensity.value=m.density)}function s(p,m,w,_,A){m.isMeshBasicMaterial||m.isMeshLambertMaterial?r(p,m):m.isMeshToonMaterial?(r(p,m),d(p,m)):m.isMeshPhongMaterial?(r(p,m),f(p,m)):m.isMeshStandardMaterial?(r(p,m),c(p,m),m.isMeshPhysicalMaterial&&u(p,m,A)):m.isMeshMatcapMaterial?(r(p,m),g(p,m)):m.isMeshDepthMaterial?r(p,m):m.isMeshDistanceMaterial?(r(p,m),y(p,m)):m.isMeshNormalMaterial?r(p,m):m.isLineBasicMaterial?(o(p,m),m.isLineDashedMaterial&&a(p,m)):m.isPointsMaterial?l(p,m,w,_):m.isSpriteMaterial?h(p,m):m.isShadowMaterial?(p.color.value.copy(m.color),p.opacity.value=m.opacity):m.isShaderMaterial&&(m.uniformsNeedUpdate=!1)}function r(p,m){p.opacity.value=m.opacity,m.color&&p.diffuse.value.copy(m.color),m.emissive&&p.emissive.value.copy(m.emissive).multiplyScalar(m.emissiveIntensity),m.map&&(p.map.value=m.map,e(m.map,p.mapTransform)),m.alphaMap&&(p.alphaMap.value=m.alphaMap,e(m.alphaMap,p.alphaMapTransform)),m.bumpMap&&(p.bumpMap.value=m.bumpMap,e(m.bumpMap,p.bumpMapTransform),p.bumpScale.value=m.bumpScale,m.side===Fe&&(p.bumpScale.value*=-1)),m.normalMap&&(p.normalMap.value=m.normalMap,e(m.normalMap,p.normalMapTransform),p.normalScale.value.copy(m.normalScale),m.side===Fe&&p.normalScale.value.negate()),m.displacementMap&&(p.displacementMap.value=m.displacementMap,e(m.displacementMap,p.displacementMapTransform),p.displacementScale.value=m.displacementScale,p.displacementBias.value=m.displacementBias),m.emissiveMap&&(p.emissiveMap.value=m.emissiveMap,e(m.emissiveMap,p.emissiveMapTransform)),m.specularMap&&(p.specularMap.value=m.specularMap,e(m.specularMap,p.specularMapTransform)),m.alphaTest>0&&(p.alphaTest.value=m.alphaTest);let w=t.get(m),_=w.envMap,A=w.envMapRotation;_&&(p.envMap.value=_,ui.copy(A),ui.x*=-1,ui.y*=-1,ui.z*=-1,_.isCubeTexture&&_.isRenderTargetTexture===!1&&(ui.y*=-1,ui.z*=-1),p.envMapRotation.value.setFromMatrix4(Lx.makeRotationFromEuler(ui)),p.flipEnvMap.value=_.isCubeTexture&&_.isRenderTargetTexture===!1?-1:1,p.reflectivity.value=m.reflectivity,p.ior.value=m.ior,p.refractionRatio.value=m.refractionRatio),m.lightMap&&(p.lightMap.value=m.lightMap,p.lightMapIntensity.value=m.lightMapIntensity,e(m.lightMap,p.lightMapTransform)),m.aoMap&&(p.aoMap.value=m.aoMap,p.aoMapIntensity.value=m.aoMapIntensity,e(m.aoMap,p.aoMapTransform))}function o(p,m){p.diffuse.value.copy(m.color),p.opacity.value=m.opacity,m.map&&(p.map.value=m.map,e(m.map,p.mapTransform))}function a(p,m){p.dashSize.value=m.dashSize,p.totalSize.value=m.dashSize+m.gapSize,p.scale.value=m.scale}function l(p,m,w,_){p.diffuse.value.copy(m.color),p.opacity.value=m.opacity,p.size.value=m.size*w,p.scale.value=_*.5,m.map&&(p.map.value=m.map,e(m.map,p.uvTransform)),m.alphaMap&&(p.alphaMap.value=m.alphaMap,e(m.alphaMap,p.alphaMapTransform)),m.alphaTest>0&&(p.alphaTest.value=m.alphaTest)}function h(p,m){p.diffuse.value.copy(m.color),p.opacity.value=m.opacity,p.rotation.value=m.rotation,m.map&&(p.map.value=m.map,e(m.map,p.mapTransform)),m.alphaMap&&(p.alphaMap.value=m.alphaMap,e(m.alphaMap,p.alphaMapTransform)),m.alphaTest>0&&(p.alphaTest.value=m.alphaTest)}function f(p,m){p.specular.value.copy(m.specular),p.shininess.value=Math.max(m.shininess,1e-4)}function d(p,m){m.gradientMap&&(p.gradientMap.value=m.gradientMap)}function c(p,m){p.metalness.value=m.metalness,m.metalnessMap&&(p.metalnessMap.value=m.metalnessMap,e(m.metalnessMap,p.metalnessMapTransform)),p.roughness.value=m.roughness,m.roughnessMap&&(p.roughnessMap.value=m.roughnessMap,e(m.roughnessMap,p.roughnessMapTransform)),m.envMap&&(p.envMapIntensity.value=m.envMapIntensity)}function u(p,m,w){p.ior.value=m.ior,m.sheen>0&&(p.sheenColor.value.copy(m.sheenColor).multiplyScalar(m.sheen),p.sheenRoughness.value=m.sheenRoughness,m.sheenColorMap&&(p.sheenColorMap.value=m.sheenColorMap,e(m.sheenColorMap,p.sheenColorMapTransform)),m.sheenRoughnessMap&&(p.sheenRoughnessMap.value=m.sheenRoughnessMap,e(m.sheenRoughnessMap,p.sheenRoughnessMapTransform))),m.clearcoat>0&&(p.clearcoat.value=m.clearcoat,p.clearcoatRoughness.value=m.clearcoatRoughness,m.clearcoatMap&&(p.clearcoatMap.value=m.clearcoatMap,e(m.clearcoatMap,p.clearcoatMapTransform)),m.clearcoatRoughnessMap&&(p.clearcoatRoughnessMap.value=m.clearcoatRoughnessMap,e(m.clearcoatRoughnessMap,p.clearcoatRoughnessMapTransform)),m.clearcoatNormalMap&&(p.clearcoatNormalMap.value=m.clearcoatNormalMap,e(m.clearcoatNormalMap,p.clearcoatNormalMapTransform),p.clearcoatNormalScale.value.copy(m.clearcoatNormalScale),m.side===Fe&&p.clearcoatNormalScale.value.negate())),m.dispersion>0&&(p.dispersion.value=m.dispersion),m.iridescence>0&&(p.iridescence.value=m.iridescence,p.iridescenceIOR.value=m.iridescenceIOR,p.iridescenceThicknessMinimum.value=m.iridescenceThicknessRange[0],p.iridescenceThicknessMaximum.value=m.iridescenceThicknessRange[1],m.iridescenceMap&&(p.iridescenceMap.value=m.iridescenceMap,e(m.iridescenceMap,p.iridescenceMapTransform)),m.iridescenceThicknessMap&&(p.iridescenceThicknessMap.value=m.iridescenceThicknessMap,e(m.iridescenceThicknessMap,p.iridescenceThicknessMapTransform))),m.transmission>0&&(p.transmission.value=m.transmission,p.transmissionSamplerMap.value=w.texture,p.transmissionSamplerSize.value.set(w.width,w.height),m.transmissionMap&&(p.transmissionMap.value=m.transmissionMap,e(m.transmissionMap,p.transmissionMapTransform)),p.thickness.value=m.thickness,m.thicknessMap&&(p.thicknessMap.value=m.thicknessMap,e(m.thicknessMap,p.thicknessMapTransform)),p.attenuationDistance.value=m.attenuationDistance,p.attenuationColor.value.copy(m.attenuationColor)),m.anisotropy>0&&(p.anisotropyVector.value.set(m.anisotropy*Math.cos(m.anisotropyRotation),m.anisotropy*Math.sin(m.anisotropyRotation)),m.anisotropyMap&&(p.anisotropyMap.value=m.anisotropyMap,e(m.anisotropyMap,p.anisotropyMapTransform))),p.specularIntensity.value=m.specularIntensity,p.specularColor.value.copy(m.specularColor),m.specularColorMap&&(p.specularColorMap.value=m.specularColorMap,e(m.specularColorMap,p.specularColorMapTransform)),m.specularIntensityMap&&(p.specularIntensityMap.value=m.specularIntensityMap,e(m.specularIntensityMap,p.specularIntensityMapTransform))}function g(p,m){m.matcap&&(p.matcap.value=m.matcap)}function y(p,m){let w=t.get(m).light;p.referencePosition.value.setFromMatrixPosition(w.matrixWorld),p.nearDistance.value=w.shadow.camera.near,p.farDistance.value=w.shadow.camera.far}return{refreshFogUniforms:i,refreshMaterialUniforms:s}}function Ux(n,t,e,i){let s={},r={},o=[],a=n.getParameter(n.MAX_UNIFORM_BUFFER_BINDINGS);function l(w,_){let A=_.program;i.uniformBlockBinding(w,A)}function h(w,_){let A=s[w.id];A===void 0&&(g(w),A=f(w),s[w.id]=A,w.addEventListener("dispose",p));let I=_.program;i.updateUBOMapping(w,I);let C=t.render.frame;r[w.id]!==C&&(c(w),r[w.id]=C)}function f(w){let _=d();w.__bindingPointIndex=_;let A=n.createBuffer(),I=w.__size,C=w.usage;return n.bindBuffer(n.UNIFORM_BUFFER,A),n.bufferData(n.UNIFORM_BUFFER,I,C),n.bindBuffer(n.UNIFORM_BUFFER,null),n.bindBufferBase(n.UNIFORM_BUFFER,_,A),A}function d(){for(let w=0;w<a;w++)if(o.indexOf(w)===-1)return o.push(w),w;return console.error("THREE.WebGLRenderer: Maximum number of simultaneously usable uniforms groups reached."),0}function c(w){let _=s[w.id],A=w.uniforms,I=w.__cache;n.bindBuffer(n.UNIFORM_BUFFER,_);for(let C=0,E=A.length;C<E;C++){let T=Array.isArray(A[C])?A[C]:[A[C]];for(let H=0,x=T.length;H<x;H++){let S=T[H];if(u(S,C,H,I)===!0){let O=S.__offset,W=Array.isArray(S.value)?S.value:[S.value],q=0;for(let et=0;et<W.length;et++){let F=W[et],it=y(F);typeof F=="number"||typeof F=="boolean"?(S.__data[0]=F,n.bufferSubData(n.UNIFORM_BUFFER,O+q,S.__data)):F.isMatrix3?(S.__data[0]=F.elements[0],S.__data[1]=F.elements[1],S.__data[2]=F.elements[2],S.__data[3]=0,S.__data[4]=F.elements[3],S.__data[5]=F.elements[4],S.__data[6]=F.elements[5],S.__data[7]=0,S.__data[8]=F.elements[6],S.__data[9]=F.elements[7],S.__data[10]=F.elements[8],S.__data[11]=0):(F.toArray(S.__data,q),q+=it.storage/Float32Array.BYTES_PER_ELEMENT)}n.bufferSubData(n.UNIFORM_BUFFER,O,S.__data)}}}n.bindBuffer(n.UNIFORM_BUFFER,null)}function u(w,_,A,I){let C=w.value,E=_+"_"+A;if(I[E]===void 0)return typeof C=="number"||typeof C=="boolean"?I[E]=C:I[E]=C.clone(),!0;{let T=I[E];if(typeof C=="number"||typeof C=="boolean"){if(T!==C)return I[E]=C,!0}else if(T.equals(C)===!1)return T.copy(C),!0}return!1}function g(w){let _=w.uniforms,A=0,I=16;for(let E=0,T=_.length;E<T;E++){let H=Array.isArray(_[E])?_[E]:[_[E]];for(let x=0,S=H.length;x<S;x++){let O=H[x],W=Array.isArray(O.value)?O.value:[O.value];for(let q=0,et=W.length;q<et;q++){let F=W[q],it=y(F),Z=A%I,gt=Z%it.boundary,_t=Z+gt;A+=gt,_t!==0&&I-_t<it.storage&&(A+=I-_t),O.__data=new Float32Array(it.storage/Float32Array.BYTES_PER_ELEMENT),O.__offset=A,A+=it.storage}}}let C=A%I;return C>0&&(A+=I-C),w.__size=A,w.__cache={},this}function y(w){let _={boundary:0,storage:0};return typeof w=="number"||typeof w=="boolean"?(_.boundary=4,_.storage=4):w.isVector2?(_.boundary=8,_.storage=8):w.isVector3||w.isColor?(_.boundary=16,_.storage=12):w.isVector4?(_.boundary=16,_.storage=16):w.isMatrix3?(_.boundary=48,_.storage=48):w.isMatrix4?(_.boundary=64,_.storage=64):w.isTexture?console.warn("THREE.WebGLRenderer: Texture samplers can not be part of an uniforms group."):console.warn("THREE.WebGLRenderer: Unsupported uniform value type.",w),_}function p(w){let _=w.target;_.removeEventListener("dispose",p);let A=o.indexOf(_.__bindingPointIndex);o.splice(A,1),n.deleteBuffer(s[_.id]),delete s[_.id],delete r[_.id]}function m(){for(let w in s)n.deleteBuffer(s[w]);o=[],s={},r={}}return{bind:l,update:h,dispose:m}}var uo=class{constructor(t={}){let{canvas:e=Bf(),context:i=null,depth:s=!0,stencil:r=!1,alpha:o=!1,antialias:a=!1,premultipliedAlpha:l=!0,preserveDrawingBuffer:h=!1,powerPreference:f="default",failIfMajorPerformanceCaveat:d=!1}=t;this.isWebGLRenderer=!0;let c;if(i!==null){if(typeof WebGLRenderingContext<"u"&&i instanceof WebGLRenderingContext)throw new Error("THREE.WebGLRenderer: WebGL 1 is not supported since r163.");c=i.getContextAttributes().alpha}else c=o;let u=new Uint32Array(4),g=new Int32Array(4),y=null,p=null,m=[],w=[];this.domElement=e,this.debug={checkShaderErrors:!0,onShaderError:null},this.autoClear=!0,this.autoClearColor=!0,this.autoClearDepth=!0,this.autoClearStencil=!0,this.sortObjects=!0,this.clippingPlanes=[],this.localClippingEnabled=!1,this._outputColorSpace=mn,this.toneMapping=Zn,this.toneMappingExposure=1;let _=this,A=!1,I=0,C=0,E=null,T=-1,H=null,x=new oe,S=new oe,O=null,W=new Ot(0),q=0,et=e.width,F=e.height,it=1,Z=null,gt=null,_t=new oe(0,0,et,F),St=new oe(0,0,et,F),Gt=!1,Wt=new nr,J=!1,st=!1,Mt=new $t,xt=new $t,Nt=new U,It=new oe,kt={background:null,fog:null,environment:null,overrideMaterial:null,isScene:!0},Zt=!1;function Vt(){return E===null?it:1}let P=i;function fe(b,D){return e.getContext(b,D)}try{let b={alpha:!0,depth:s,stencil:r,antialias:a,premultipliedAlpha:l,preserveDrawingBuffer:h,powerPreference:f,failIfMajorPerformanceCaveat:d};if("setAttribute"in e&&e.setAttribute("data-engine","three.js r169"),e.addEventListener("webglcontextlost",$,!1),e.addEventListener("webglcontextrestored",pt,!1),e.addEventListener("webglcontextcreationerror",vt,!1),P===null){let D="webgl2";if(P=fe(D,b),P===null)throw fe(D)?new Error("Error creating WebGL context with your selected attributes."):new Error("Error creating WebGL context.")}}catch(b){throw console.error("THREE.WebGLRenderer: "+b.message),b}let j,lt,ut,Rt,B,M,v,R,k,K,z,nt,Q,X,ht,tt,ct,At,rt,ot,Lt,wt,Ft,L;function ft(){j=new Kg(P),j.init(),wt=new Cx(P,j),lt=new Wg(P,j,t,wt),ut=new Ax(P),lt.reverseDepthBuffer&&ut.buffers.depth.setReversed(!0),Rt=new $g(P),B=new dx,M=new Tx(P,j,ut,B,lt,wt,Rt),v=new qg(_),R=new jg(_),k=new op(P),Ft=new Vg(P,k),K=new Jg(P,k,Rt,Ft),z=new e0(P,K,k,Rt),rt=new t0(P,lt,M),tt=new Xg(B),nt=new ux(_,v,R,j,lt,Ft,tt),Q=new Dx(_,B),X=new px,ht=new yx(j),At=new Hg(_,v,R,ut,z,c,l),ct=new bx(_,z,lt),L=new Ux(P,Rt,lt,ut),ot=new Gg(P,j,Rt),Lt=new Qg(P,j,Rt),Rt.programs=nt.programs,_.capabilities=lt,_.extensions=j,_.properties=B,_.renderLists=X,_.shadowMap=ct,_.state=ut,_.info=Rt}ft();let Y=new Gl(_,P);this.xr=Y,this.getContext=function(){return P},this.getContextAttributes=function(){return P.getContextAttributes()},this.forceContextLoss=function(){let b=j.get("WEBGL_lose_context");b&&b.loseContext()},this.forceContextRestore=function(){let b=j.get("WEBGL_lose_context");b&&b.restoreContext()},this.getPixelRatio=function(){return it},this.setPixelRatio=function(b){b!==void 0&&(it=b,this.setSize(et,F,!1))},this.getSize=function(b){return b.set(et,F)},this.setSize=function(b,D,V=!0){if(Y.isPresenting){console.warn("THREE.WebGLRenderer: Can't change size while VR device is presenting.");return}et=b,F=D,e.width=Math.floor(b*it),e.height=Math.floor(D*it),V===!0&&(e.style.width=b+"px",e.style.height=D+"px"),this.setViewport(0,0,b,D)},this.getDrawingBufferSize=function(b){return b.set(et*it,F*it).floor()},this.setDrawingBufferSize=function(b,D,V){et=b,F=D,it=V,e.width=Math.floor(b*V),e.height=Math.floor(D*V),this.setViewport(0,0,b,D)},this.getCurrentViewport=function(b){return b.copy(x)},this.getViewport=function(b){return b.copy(_t)},this.setViewport=function(b,D,V,G){b.isVector4?_t.set(b.x,b.y,b.z,b.w):_t.set(b,D,V,G),ut.viewport(x.copy(_t).multiplyScalar(it).round())},this.getScissor=function(b){return b.copy(St)},this.setScissor=function(b,D,V,G){b.isVector4?St.set(b.x,b.y,b.z,b.w):St.set(b,D,V,G),ut.scissor(S.copy(St).multiplyScalar(it).round())},this.getScissorTest=function(){return Gt},this.setScissorTest=function(b){ut.setScissorTest(Gt=b)},this.setOpaqueSort=function(b){Z=b},this.setTransparentSort=function(b){gt=b},this.getClearColor=function(b){return b.copy(At.getClearColor())},this.setClearColor=function(){At.setClearColor.apply(At,arguments)},this.getClearAlpha=function(){return At.getClearAlpha()},this.setClearAlpha=function(){At.setClearAlpha.apply(At,arguments)},this.clear=function(b=!0,D=!0,V=!0){let G=0;if(b){let N=!1;if(E!==null){let at=E.texture.format;N=at===uc||at===hc||at===cc}if(N){let at=E.texture.type,mt=at===In||at===vi||at===Qs||at===us||at===oc||at===ac,bt=At.getClearColor(),Et=At.getClearAlpha(),Dt=bt.r,Ut=bt.g,Tt=bt.b;mt?(u[0]=Dt,u[1]=Ut,u[2]=Tt,u[3]=Et,P.clearBufferuiv(P.COLOR,0,u)):(g[0]=Dt,g[1]=Ut,g[2]=Tt,g[3]=Et,P.clearBufferiv(P.COLOR,0,g))}else G|=P.COLOR_BUFFER_BIT}D&&(G|=P.DEPTH_BUFFER_BIT,P.clearDepth(this.capabilities.reverseDepthBuffer?0:1)),V&&(G|=P.STENCIL_BUFFER_BIT,this.state.buffers.stencil.setMask(4294967295)),P.clear(G)},this.clearColor=function(){this.clear(!0,!1,!1)},this.clearDepth=function(){this.clear(!1,!0,!1)},this.clearStencil=function(){this.clear(!1,!1,!0)},this.dispose=function(){e.removeEventListener("webglcontextlost",$,!1),e.removeEventListener("webglcontextrestored",pt,!1),e.removeEventListener("webglcontextcreationerror",vt,!1),X.dispose(),ht.dispose(),B.dispose(),v.dispose(),R.dispose(),z.dispose(),Ft.dispose(),L.dispose(),nt.dispose(),Y.dispose(),Y.removeEventListener("sessionstart",Jc),Y.removeEventListener("sessionend",Qc),ri.stop()};function $(b){b.preventDefault(),console.log("THREE.WebGLRenderer: Context Lost."),A=!0}function pt(){console.log("THREE.WebGLRenderer: Context Restored."),A=!1;let b=Rt.autoReset,D=ct.enabled,V=ct.autoUpdate,G=ct.needsUpdate,N=ct.type;ft(),Rt.autoReset=b,ct.enabled=D,ct.autoUpdate=V,ct.needsUpdate=G,ct.type=N}function vt(b){console.error("THREE.WebGLRenderer: A WebGL context could not be created. Reason: ",b.statusMessage)}function Xt(b){let D=b.target;D.removeEventListener("dispose",Xt),ce(D)}function ce(b){Ne(b),B.remove(b)}function Ne(b){let D=B.get(b).programs;D!==void 0&&(D.forEach(function(V){nt.releaseProgram(V)}),b.isShaderMaterial&&nt.releaseShaderCache(b))}this.renderBufferDirect=function(b,D,V,G,N,at){D===null&&(D=kt);let mt=N.isMesh&&N.matrixWorld.determinant()<0,bt=Pd(b,D,V,G,N);ut.setMaterial(G,mt);let Et=V.index,Dt=1;if(G.wireframe===!0){if(Et=K.getWireframeAttribute(V),Et===void 0)return;Dt=2}let Ut=V.drawRange,Tt=V.attributes.position,Qt=Ut.start*Dt,te=(Ut.start+Ut.count)*Dt;at!==null&&(Qt=Math.max(Qt,at.start*Dt),te=Math.min(te,(at.start+at.count)*Dt)),Et!==null?(Qt=Math.max(Qt,0),te=Math.min(te,Et.count)):Tt!=null&&(Qt=Math.max(Qt,0),te=Math.min(te,Tt.count));let re=te-Qt;if(re<0||re===1/0)return;Ft.setup(N,G,bt,V,Et);let He,jt=ot;if(Et!==null&&(He=k.get(Et),jt=Lt,jt.setIndex(He)),N.isMesh)G.wireframe===!0?(ut.setLineWidth(G.wireframeLinewidth*Vt()),jt.setMode(P.LINES)):jt.setMode(P.TRIANGLES);else if(N.isLine){let Ct=G.linewidth;Ct===void 0&&(Ct=1),ut.setLineWidth(Ct*Vt()),N.isLineSegments?jt.setMode(P.LINES):N.isLineLoop?jt.setMode(P.LINE_LOOP):jt.setMode(P.LINE_STRIP)}else N.isPoints?jt.setMode(P.POINTS):N.isSprite&&jt.setMode(P.TRIANGLES);if(N.isBatchedMesh)if(N._multiDrawInstances!==null)jt.renderMultiDrawInstances(N._multiDrawStarts,N._multiDrawCounts,N._multiDrawCount,N._multiDrawInstances);else if(j.get("WEBGL_multi_draw"))jt.renderMultiDraw(N._multiDrawStarts,N._multiDrawCounts,N._multiDrawCount);else{let Ct=N._multiDrawStarts,Se=N._multiDrawCounts,Kt=N._multiDrawCount,rn=Et?k.get(Et).bytesPerElement:1,zi=B.get(G).currentProgram.getUniforms();for(let Ve=0;Ve<Kt;Ve++)zi.setValue(P,"_gl_DrawID",Ve),jt.render(Ct[Ve]/rn,Se[Ve])}else if(N.isInstancedMesh)jt.renderInstances(Qt,re,N.count);else if(V.isInstancedBufferGeometry){let Ct=V._maxInstanceCount!==void 0?V._maxInstanceCount:1/0,Se=Math.min(V.instanceCount,Ct);jt.renderInstances(Qt,re,Se)}else jt.render(Qt,re)};function qt(b,D,V){b.transparent===!0&&b.side===Rn&&b.forceSinglePass===!1?(b.side=Fe,b.needsUpdate=!0,_r(b,D,V),b.side=jn,b.needsUpdate=!0,_r(b,D,V),b.side=Rn):_r(b,D,V)}this.compile=function(b,D,V=null){V===null&&(V=b),p=ht.get(V),p.init(D),w.push(p),V.traverseVisible(function(N){N.isLight&&N.layers.test(D.layers)&&(p.pushLight(N),N.castShadow&&p.pushShadow(N))}),b!==V&&b.traverseVisible(function(N){N.isLight&&N.layers.test(D.layers)&&(p.pushLight(N),N.castShadow&&p.pushShadow(N))}),p.setupLights();let G=new Set;return b.traverse(function(N){if(!(N.isMesh||N.isPoints||N.isLine||N.isSprite))return;let at=N.material;if(at)if(Array.isArray(at))for(let mt=0;mt<at.length;mt++){let bt=at[mt];qt(bt,V,N),G.add(bt)}else qt(at,V,N),G.add(at)}),w.pop(),p=null,G},this.compileAsync=function(b,D,V=null){let G=this.compile(b,D,V);return new Promise(N=>{function at(){if(G.forEach(function(mt){B.get(mt).currentProgram.isReady()&&G.delete(mt)}),G.size===0){N(b);return}setTimeout(at,10)}j.get("KHR_parallel_shader_compile")!==null?at():setTimeout(at,10)})};let Oe=null;function Sn(b){Oe&&Oe(b)}function Jc(){ri.stop()}function Qc(){ri.start()}let ri=new Au;ri.setAnimationLoop(Sn),typeof self<"u"&&ri.setContext(self),this.setAnimationLoop=function(b){Oe=b,Y.setAnimationLoop(b),b===null?ri.stop():ri.start()},Y.addEventListener("sessionstart",Jc),Y.addEventListener("sessionend",Qc),this.render=function(b,D){if(D!==void 0&&D.isCamera!==!0){console.error("THREE.WebGLRenderer.render: camera is not an instance of THREE.Camera.");return}if(A===!0)return;if(b.matrixWorldAutoUpdate===!0&&b.updateMatrixWorld(),D.parent===null&&D.matrixWorldAutoUpdate===!0&&D.updateMatrixWorld(),Y.enabled===!0&&Y.isPresenting===!0&&(Y.cameraAutoUpdate===!0&&Y.updateCamera(D),D=Y.getCamera()),b.isScene===!0&&b.onBeforeRender(_,b,D,E),p=ht.get(b,w.length),p.init(D),w.push(p),xt.multiplyMatrices(D.projectionMatrix,D.matrixWorldInverse),Wt.setFromProjectionMatrix(xt),st=this.localClippingEnabled,J=tt.init(this.clippingPlanes,st),y=X.get(b,m.length),y.init(),m.push(y),Y.enabled===!0&&Y.isPresenting===!0){let at=_.xr.getDepthSensingMesh();at!==null&&la(at,D,-1/0,_.sortObjects)}la(b,D,0,_.sortObjects),y.finish(),_.sortObjects===!0&&y.sort(Z,gt),Zt=Y.enabled===!1||Y.isPresenting===!1||Y.hasDepthSensing()===!1,Zt&&At.addToRenderList(y,b),this.info.render.frame++,J===!0&&tt.beginShadows();let V=p.state.shadowsArray;ct.render(V,b,D),J===!0&&tt.endShadows(),this.info.autoReset===!0&&this.info.reset();let G=y.opaque,N=y.transmissive;if(p.setupLights(),D.isArrayCamera){let at=D.cameras;if(N.length>0)for(let mt=0,bt=at.length;mt<bt;mt++){let Et=at[mt];th(G,N,b,Et)}Zt&&At.render(b);for(let mt=0,bt=at.length;mt<bt;mt++){let Et=at[mt];$c(y,b,Et,Et.viewport)}}else N.length>0&&th(G,N,b,D),Zt&&At.render(b),$c(y,b,D);E!==null&&(M.updateMultisampleRenderTarget(E),M.updateRenderTargetMipmap(E)),b.isScene===!0&&b.onAfterRender(_,b,D),Ft.resetDefaultState(),T=-1,H=null,w.pop(),w.length>0?(p=w[w.length-1],J===!0&&tt.setGlobalState(_.clippingPlanes,p.state.camera)):p=null,m.pop(),m.length>0?y=m[m.length-1]:y=null};function la(b,D,V,G){if(b.visible===!1)return;if(b.layers.test(D.layers)){if(b.isGroup)V=b.renderOrder;else if(b.isLOD)b.autoUpdate===!0&&b.update(D);else if(b.isLight)p.pushLight(b),b.castShadow&&p.pushShadow(b);else if(b.isSprite){if(!b.frustumCulled||Wt.intersectsSprite(b)){G&&It.setFromMatrixPosition(b.matrixWorld).applyMatrix4(xt);let mt=z.update(b),bt=b.material;bt.visible&&y.push(b,mt,bt,V,It.z,null)}}else if((b.isMesh||b.isLine||b.isPoints)&&(!b.frustumCulled||Wt.intersectsObject(b))){let mt=z.update(b),bt=b.material;if(G&&(b.boundingSphere!==void 0?(b.boundingSphere===null&&b.computeBoundingSphere(),It.copy(b.boundingSphere.center)):(mt.boundingSphere===null&&mt.computeBoundingSphere(),It.copy(mt.boundingSphere.center)),It.applyMatrix4(b.matrixWorld).applyMatrix4(xt)),Array.isArray(bt)){let Et=mt.groups;for(let Dt=0,Ut=Et.length;Dt<Ut;Dt++){let Tt=Et[Dt],Qt=bt[Tt.materialIndex];Qt&&Qt.visible&&y.push(b,mt,Qt,V,It.z,Tt)}}else bt.visible&&y.push(b,mt,bt,V,It.z,null)}}let at=b.children;for(let mt=0,bt=at.length;mt<bt;mt++)la(at[mt],D,V,G)}function $c(b,D,V,G){let N=b.opaque,at=b.transmissive,mt=b.transparent;p.setupLightsView(V),J===!0&&tt.setGlobalState(_.clippingPlanes,V),G&&ut.viewport(x.copy(G)),N.length>0&&vr(N,D,V),at.length>0&&vr(at,D,V),mt.length>0&&vr(mt,D,V),ut.buffers.depth.setTest(!0),ut.buffers.depth.setMask(!0),ut.buffers.color.setMask(!0),ut.setPolygonOffset(!1)}function th(b,D,V,G){if((V.isScene===!0?V.overrideMaterial:null)!==null)return;p.state.transmissionRenderTarget[G.id]===void 0&&(p.state.transmissionRenderTarget[G.id]=new ue(1,1,{generateMipmaps:!0,type:j.has("EXT_color_buffer_half_float")||j.has("EXT_color_buffer_float")?ze:In,minFilter:xi,samples:4,stencilBuffer:r,resolveDepthBuffer:!1,resolveStencilBuffer:!1,colorSpace:Jt.workingColorSpace}));let at=p.state.transmissionRenderTarget[G.id],mt=G.viewport||x;at.setSize(mt.z,mt.w);let bt=_.getRenderTarget();_.setRenderTarget(at),_.getClearColor(W),q=_.getClearAlpha(),q<1&&_.setClearColor(16777215,.5),_.clear(),Zt&&At.render(V);let Et=_.toneMapping;_.toneMapping=Zn;let Dt=G.viewport;if(G.viewport!==void 0&&(G.viewport=void 0),p.setupLightsView(G),J===!0&&tt.setGlobalState(_.clippingPlanes,G),vr(b,V,G),M.updateMultisampleRenderTarget(at),M.updateRenderTargetMipmap(at),j.has("WEBGL_multisampled_render_to_texture")===!1){let Ut=!1;for(let Tt=0,Qt=D.length;Tt<Qt;Tt++){let te=D[Tt],re=te.object,He=te.geometry,jt=te.material,Ct=te.group;if(jt.side===Rn&&re.layers.test(G.layers)){let Se=jt.side;jt.side=Fe,jt.needsUpdate=!0,eh(re,V,G,He,jt,Ct),jt.side=Se,jt.needsUpdate=!0,Ut=!0}}Ut===!0&&(M.updateMultisampleRenderTarget(at),M.updateRenderTargetMipmap(at))}_.setRenderTarget(bt),_.setClearColor(W,q),Dt!==void 0&&(G.viewport=Dt),_.toneMapping=Et}function vr(b,D,V){let G=D.isScene===!0?D.overrideMaterial:null;for(let N=0,at=b.length;N<at;N++){let mt=b[N],bt=mt.object,Et=mt.geometry,Dt=G===null?mt.material:G,Ut=mt.group;bt.layers.test(V.layers)&&eh(bt,D,V,Et,Dt,Ut)}}function eh(b,D,V,G,N,at){b.onBeforeRender(_,D,V,G,N,at),b.modelViewMatrix.multiplyMatrices(V.matrixWorldInverse,b.matrixWorld),b.normalMatrix.getNormalMatrix(b.modelViewMatrix),N.onBeforeRender(_,D,V,G,b,at),N.transparent===!0&&N.side===Rn&&N.forceSinglePass===!1?(N.side=Fe,N.needsUpdate=!0,_.renderBufferDirect(V,D,G,N,b,at),N.side=jn,N.needsUpdate=!0,_.renderBufferDirect(V,D,G,N,b,at),N.side=Rn):_.renderBufferDirect(V,D,G,N,b,at),b.onAfterRender(_,D,V,G,N,at)}function _r(b,D,V){D.isScene!==!0&&(D=kt);let G=B.get(b),N=p.state.lights,at=p.state.shadowsArray,mt=N.state.version,bt=nt.getParameters(b,N.state,at,D,V),Et=nt.getProgramCacheKey(bt),Dt=G.programs;G.environment=b.isMeshStandardMaterial?D.environment:null,G.fog=D.fog,G.envMap=(b.isMeshStandardMaterial?R:v).get(b.envMap||G.environment),G.envMapRotation=G.environment!==null&&b.envMap===null?D.environmentRotation:b.envMapRotation,Dt===void 0&&(b.addEventListener("dispose",Xt),Dt=new Map,G.programs=Dt);let Ut=Dt.get(Et);if(Ut!==void 0){if(G.currentProgram===Ut&&G.lightsStateVersion===mt)return ih(b,bt),Ut}else bt.uniforms=nt.getUniforms(b),b.onBeforeCompile(bt,_),Ut=nt.acquireProgram(bt,Et),Dt.set(Et,Ut),G.uniforms=bt.uniforms;let Tt=G.uniforms;return(!b.isShaderMaterial&&!b.isRawShaderMaterial||b.clipping===!0)&&(Tt.clippingPlanes=tt.uniform),ih(b,bt),G.needsLights=Ld(b),G.lightsStateVersion=mt,G.needsLights&&(Tt.ambientLightColor.value=N.state.ambient,Tt.lightProbe.value=N.state.probe,Tt.directionalLights.value=N.state.directional,Tt.directionalLightShadows.value=N.state.directionalShadow,Tt.spotLights.value=N.state.spot,Tt.spotLightShadows.value=N.state.spotShadow,Tt.rectAreaLights.value=N.state.rectArea,Tt.ltc_1.value=N.state.rectAreaLTC1,Tt.ltc_2.value=N.state.rectAreaLTC2,Tt.pointLights.value=N.state.point,Tt.pointLightShadows.value=N.state.pointShadow,Tt.hemisphereLights.value=N.state.hemi,Tt.directionalShadowMap.value=N.state.directionalShadowMap,Tt.directionalShadowMatrix.value=N.state.directionalShadowMatrix,Tt.spotShadowMap.value=N.state.spotShadowMap,Tt.spotLightMatrix.value=N.state.spotLightMatrix,Tt.spotLightMap.value=N.state.spotLightMap,Tt.pointShadowMap.value=N.state.pointShadowMap,Tt.pointShadowMatrix.value=N.state.pointShadowMatrix),G.currentProgram=Ut,G.uniformsList=null,Ut}function nh(b){if(b.uniformsList===null){let D=b.currentProgram.getUniforms();b.uniformsList=os.seqWithValue(D.seq,b.uniforms)}return b.uniformsList}function ih(b,D){let V=B.get(b);V.outputColorSpace=D.outputColorSpace,V.batching=D.batching,V.batchingColor=D.batchingColor,V.instancing=D.instancing,V.instancingColor=D.instancingColor,V.instancingMorph=D.instancingMorph,V.skinning=D.skinning,V.morphTargets=D.morphTargets,V.morphNormals=D.morphNormals,V.morphColors=D.morphColors,V.morphTargetsCount=D.morphTargetsCount,V.numClippingPlanes=D.numClippingPlanes,V.numIntersection=D.numClipIntersection,V.vertexAlphas=D.vertexAlphas,V.vertexTangents=D.vertexTangents,V.toneMapping=D.toneMapping}function Pd(b,D,V,G,N){D.isScene!==!0&&(D=kt),M.resetTextureUnits();let at=D.fog,mt=G.isMeshStandardMaterial?D.environment:null,bt=E===null?_.outputColorSpace:E.isXRRenderTarget===!0?E.texture.colorSpace:Kn,Et=(G.isMeshStandardMaterial?R:v).get(G.envMap||mt),Dt=G.vertexColors===!0&&!!V.attributes.color&&V.attributes.color.itemSize===4,Ut=!!V.attributes.tangent&&(!!G.normalMap||G.anisotropy>0),Tt=!!V.morphAttributes.position,Qt=!!V.morphAttributes.normal,te=!!V.morphAttributes.color,re=Zn;G.toneMapped&&(E===null||E.isXRRenderTarget===!0)&&(re=_.toneMapping);let He=V.morphAttributes.position||V.morphAttributes.normal||V.morphAttributes.color,jt=He!==void 0?He.length:0,Ct=B.get(G),Se=p.state.lights;if(J===!0&&(st===!0||b!==H)){let Je=b===H&&G.id===T;tt.setState(G,b,Je)}let Kt=!1;G.version===Ct.__version?(Ct.needsLights&&Ct.lightsStateVersion!==Se.state.version||Ct.outputColorSpace!==bt||N.isBatchedMesh&&Ct.batching===!1||!N.isBatchedMesh&&Ct.batching===!0||N.isBatchedMesh&&Ct.batchingColor===!0&&N.colorTexture===null||N.isBatchedMesh&&Ct.batchingColor===!1&&N.colorTexture!==null||N.isInstancedMesh&&Ct.instancing===!1||!N.isInstancedMesh&&Ct.instancing===!0||N.isSkinnedMesh&&Ct.skinning===!1||!N.isSkinnedMesh&&Ct.skinning===!0||N.isInstancedMesh&&Ct.instancingColor===!0&&N.instanceColor===null||N.isInstancedMesh&&Ct.instancingColor===!1&&N.instanceColor!==null||N.isInstancedMesh&&Ct.instancingMorph===!0&&N.morphTexture===null||N.isInstancedMesh&&Ct.instancingMorph===!1&&N.morphTexture!==null||Ct.envMap!==Et||G.fog===!0&&Ct.fog!==at||Ct.numClippingPlanes!==void 0&&(Ct.numClippingPlanes!==tt.numPlanes||Ct.numIntersection!==tt.numIntersection)||Ct.vertexAlphas!==Dt||Ct.vertexTangents!==Ut||Ct.morphTargets!==Tt||Ct.morphNormals!==Qt||Ct.morphColors!==te||Ct.toneMapping!==re||Ct.morphTargetsCount!==jt)&&(Kt=!0):(Kt=!0,Ct.__version=G.version);let rn=Ct.currentProgram;Kt===!0&&(rn=_r(G,D,N));let zi=!1,Ve=!1,ca=!1,le=rn.getUniforms(),zn=Ct.uniforms;if(ut.useProgram(rn.program)&&(zi=!0,Ve=!0,ca=!0),G.id!==T&&(T=G.id,Ve=!0),zi||H!==b){lt.reverseDepthBuffer?(Mt.copy(b.projectionMatrix),kf(Mt),Hf(Mt),le.setValue(P,"projectionMatrix",Mt)):le.setValue(P,"projectionMatrix",b.projectionMatrix),le.setValue(P,"viewMatrix",b.matrixWorldInverse);let Je=le.map.cameraPosition;Je!==void 0&&Je.setValue(P,Nt.setFromMatrixPosition(b.matrixWorld)),lt.logarithmicDepthBuffer&&le.setValue(P,"logDepthBufFC",2/(Math.log(b.far+1)/Math.LN2)),(G.isMeshPhongMaterial||G.isMeshToonMaterial||G.isMeshLambertMaterial||G.isMeshBasicMaterial||G.isMeshStandardMaterial||G.isShaderMaterial)&&le.setValue(P,"isOrthographic",b.isOrthographicCamera===!0),H!==b&&(H=b,Ve=!0,ca=!0)}if(N.isSkinnedMesh){le.setOptional(P,N,"bindMatrix"),le.setOptional(P,N,"bindMatrixInverse");let Je=N.skeleton;Je&&(Je.boneTexture===null&&Je.computeBoneTexture(),le.setValue(P,"boneTexture",Je.boneTexture,M))}N.isBatchedMesh&&(le.setOptional(P,N,"batchingTexture"),le.setValue(P,"batchingTexture",N._matricesTexture,M),le.setOptional(P,N,"batchingIdTexture"),le.setValue(P,"batchingIdTexture",N._indirectTexture,M),le.setOptional(P,N,"batchingColorTexture"),N._colorsTexture!==null&&le.setValue(P,"batchingColorTexture",N._colorsTexture,M));let ha=V.morphAttributes;if((ha.position!==void 0||ha.normal!==void 0||ha.color!==void 0)&&rt.update(N,V,rn),(Ve||Ct.receiveShadow!==N.receiveShadow)&&(Ct.receiveShadow=N.receiveShadow,le.setValue(P,"receiveShadow",N.receiveShadow)),G.isMeshGouraudMaterial&&G.envMap!==null&&(zn.envMap.value=Et,zn.flipEnvMap.value=Et.isCubeTexture&&Et.isRenderTargetTexture===!1?-1:1),G.isMeshStandardMaterial&&G.envMap===null&&D.environment!==null&&(zn.envMapIntensity.value=D.environmentIntensity),Ve&&(le.setValue(P,"toneMappingExposure",_.toneMappingExposure),Ct.needsLights&&Id(zn,ca),at&&G.fog===!0&&Q.refreshFogUniforms(zn,at),Q.refreshMaterialUniforms(zn,G,it,F,p.state.transmissionRenderTarget[b.id]),os.upload(P,nh(Ct),zn,M)),G.isShaderMaterial&&G.uniformsNeedUpdate===!0&&(os.upload(P,nh(Ct),zn,M),G.uniformsNeedUpdate=!1),G.isSpriteMaterial&&le.setValue(P,"center",N.center),le.setValue(P,"modelViewMatrix",N.modelViewMatrix),le.setValue(P,"normalMatrix",N.normalMatrix),le.setValue(P,"modelMatrix",N.matrixWorld),G.isShaderMaterial||G.isRawShaderMaterial){let Je=G.uniformsGroups;for(let ua=0,Dd=Je.length;ua<Dd;ua++){let sh=Je[ua];L.update(sh,rn),L.bind(sh,rn)}}return rn}function Id(b,D){b.ambientLightColor.needsUpdate=D,b.lightProbe.needsUpdate=D,b.directionalLights.needsUpdate=D,b.directionalLightShadows.needsUpdate=D,b.pointLights.needsUpdate=D,b.pointLightShadows.needsUpdate=D,b.spotLights.needsUpdate=D,b.spotLightShadows.needsUpdate=D,b.rectAreaLights.needsUpdate=D,b.hemisphereLights.needsUpdate=D}function Ld(b){return b.isMeshLambertMaterial||b.isMeshToonMaterial||b.isMeshPhongMaterial||b.isMeshStandardMaterial||b.isShadowMaterial||b.isShaderMaterial&&b.lights===!0}this.getActiveCubeFace=function(){return I},this.getActiveMipmapLevel=function(){return C},this.getRenderTarget=function(){return E},this.setRenderTargetTextures=function(b,D,V){B.get(b.texture).__webglTexture=D,B.get(b.depthTexture).__webglTexture=V;let G=B.get(b);G.__hasExternalTextures=!0,G.__autoAllocateDepthBuffer=V===void 0,G.__autoAllocateDepthBuffer||j.has("WEBGL_multisampled_render_to_texture")===!0&&(console.warn("THREE.WebGLRenderer: Render-to-texture extension was disabled because an external texture was provided"),G.__useRenderToTexture=!1)},this.setRenderTargetFramebuffer=function(b,D){let V=B.get(b);V.__webglFramebuffer=D,V.__useDefaultFramebuffer=D===void 0},this.setRenderTarget=function(b,D=0,V=0){E=b,I=D,C=V;let G=!0,N=null,at=!1,mt=!1;if(b){let Et=B.get(b);if(Et.__useDefaultFramebuffer!==void 0)ut.bindFramebuffer(P.FRAMEBUFFER,null),G=!1;else if(Et.__webglFramebuffer===void 0)M.setupRenderTarget(b);else if(Et.__hasExternalTextures)M.rebindTextures(b,B.get(b.texture).__webglTexture,B.get(b.depthTexture).__webglTexture);else if(b.depthBuffer){let Tt=b.depthTexture;if(Et.__boundDepthTexture!==Tt){if(Tt!==null&&B.has(Tt)&&(b.width!==Tt.image.width||b.height!==Tt.image.height))throw new Error("WebGLRenderTarget: Attached DepthTexture is initialized to the incorrect size.");M.setupDepthRenderbuffer(b)}}let Dt=b.texture;(Dt.isData3DTexture||Dt.isDataArrayTexture||Dt.isCompressedArrayTexture)&&(mt=!0);let Ut=B.get(b).__webglFramebuffer;b.isWebGLCubeRenderTarget?(Array.isArray(Ut[D])?N=Ut[D][V]:N=Ut[D],at=!0):b.samples>0&&M.useMultisampledRTT(b)===!1?N=B.get(b).__webglMultisampledFramebuffer:Array.isArray(Ut)?N=Ut[V]:N=Ut,x.copy(b.viewport),S.copy(b.scissor),O=b.scissorTest}else x.copy(_t).multiplyScalar(it).floor(),S.copy(St).multiplyScalar(it).floor(),O=Gt;if(ut.bindFramebuffer(P.FRAMEBUFFER,N)&&G&&ut.drawBuffers(b,N),ut.viewport(x),ut.scissor(S),ut.setScissorTest(O),at){let Et=B.get(b.texture);P.framebufferTexture2D(P.FRAMEBUFFER,P.COLOR_ATTACHMENT0,P.TEXTURE_CUBE_MAP_POSITIVE_X+D,Et.__webglTexture,V)}else if(mt){let Et=B.get(b.texture),Dt=D||0;P.framebufferTextureLayer(P.FRAMEBUFFER,P.COLOR_ATTACHMENT0,Et.__webglTexture,V||0,Dt)}T=-1},this.readRenderTargetPixels=function(b,D,V,G,N,at,mt){if(!(b&&b.isWebGLRenderTarget)){console.error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");return}let bt=B.get(b).__webglFramebuffer;if(b.isWebGLCubeRenderTarget&&mt!==void 0&&(bt=bt[mt]),bt){ut.bindFramebuffer(P.FRAMEBUFFER,bt);try{let Et=b.texture,Dt=Et.format,Ut=Et.type;if(!lt.textureFormatReadable(Dt)){console.error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not in RGBA or implementation defined format.");return}if(!lt.textureTypeReadable(Ut)){console.error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not in UnsignedByteType or implementation defined type.");return}D>=0&&D<=b.width-G&&V>=0&&V<=b.height-N&&P.readPixels(D,V,G,N,wt.convert(Dt),wt.convert(Ut),at)}finally{let Et=E!==null?B.get(E).__webglFramebuffer:null;ut.bindFramebuffer(P.FRAMEBUFFER,Et)}}},this.readRenderTargetPixelsAsync=async function(b,D,V,G,N,at,mt){if(!(b&&b.isWebGLRenderTarget))throw new Error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");let bt=B.get(b).__webglFramebuffer;if(b.isWebGLCubeRenderTarget&&mt!==void 0&&(bt=bt[mt]),bt){let Et=b.texture,Dt=Et.format,Ut=Et.type;if(!lt.textureFormatReadable(Dt))throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in RGBA or implementation defined format.");if(!lt.textureTypeReadable(Ut))throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in UnsignedByteType or implementation defined type.");if(D>=0&&D<=b.width-G&&V>=0&&V<=b.height-N){ut.bindFramebuffer(P.FRAMEBUFFER,bt);let Tt=P.createBuffer();P.bindBuffer(P.PIXEL_PACK_BUFFER,Tt),P.bufferData(P.PIXEL_PACK_BUFFER,at.byteLength,P.STREAM_READ),P.readPixels(D,V,G,N,wt.convert(Dt),wt.convert(Ut),0);let Qt=E!==null?B.get(E).__webglFramebuffer:null;ut.bindFramebuffer(P.FRAMEBUFFER,Qt);let te=P.fenceSync(P.SYNC_GPU_COMMANDS_COMPLETE,0);return P.flush(),await zf(P,te,4),P.bindBuffer(P.PIXEL_PACK_BUFFER,Tt),P.getBufferSubData(P.PIXEL_PACK_BUFFER,0,at),P.deleteBuffer(Tt),P.deleteSync(te),at}else throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: requested read bounds are out of range.")}},this.copyFramebufferToTexture=function(b,D=null,V=0){b.isTexture!==!0&&(Zr("WebGLRenderer: copyFramebufferToTexture function signature has changed."),D=arguments[0]||null,b=arguments[1]);let G=Math.pow(2,-V),N=Math.floor(b.image.width*G),at=Math.floor(b.image.height*G),mt=D!==null?D.x:0,bt=D!==null?D.y:0;M.setTexture2D(b,0),P.copyTexSubImage2D(P.TEXTURE_2D,V,0,0,mt,bt,N,at),ut.unbindTexture()},this.copyTextureToTexture=function(b,D,V=null,G=null,N=0){b.isTexture!==!0&&(Zr("WebGLRenderer: copyTextureToTexture function signature has changed."),G=arguments[0]||null,b=arguments[1],D=arguments[2],N=arguments[3]||0,V=null);let at,mt,bt,Et,Dt,Ut;V!==null?(at=V.max.x-V.min.x,mt=V.max.y-V.min.y,bt=V.min.x,Et=V.min.y):(at=b.image.width,mt=b.image.height,bt=0,Et=0),G!==null?(Dt=G.x,Ut=G.y):(Dt=0,Ut=0);let Tt=wt.convert(D.format),Qt=wt.convert(D.type);M.setTexture2D(D,0),P.pixelStorei(P.UNPACK_FLIP_Y_WEBGL,D.flipY),P.pixelStorei(P.UNPACK_PREMULTIPLY_ALPHA_WEBGL,D.premultiplyAlpha),P.pixelStorei(P.UNPACK_ALIGNMENT,D.unpackAlignment);let te=P.getParameter(P.UNPACK_ROW_LENGTH),re=P.getParameter(P.UNPACK_IMAGE_HEIGHT),He=P.getParameter(P.UNPACK_SKIP_PIXELS),jt=P.getParameter(P.UNPACK_SKIP_ROWS),Ct=P.getParameter(P.UNPACK_SKIP_IMAGES),Se=b.isCompressedTexture?b.mipmaps[N]:b.image;P.pixelStorei(P.UNPACK_ROW_LENGTH,Se.width),P.pixelStorei(P.UNPACK_IMAGE_HEIGHT,Se.height),P.pixelStorei(P.UNPACK_SKIP_PIXELS,bt),P.pixelStorei(P.UNPACK_SKIP_ROWS,Et),b.isDataTexture?P.texSubImage2D(P.TEXTURE_2D,N,Dt,Ut,at,mt,Tt,Qt,Se.data):b.isCompressedTexture?P.compressedTexSubImage2D(P.TEXTURE_2D,N,Dt,Ut,Se.width,Se.height,Tt,Se.data):P.texSubImage2D(P.TEXTURE_2D,N,Dt,Ut,at,mt,Tt,Qt,Se),P.pixelStorei(P.UNPACK_ROW_LENGTH,te),P.pixelStorei(P.UNPACK_IMAGE_HEIGHT,re),P.pixelStorei(P.UNPACK_SKIP_PIXELS,He),P.pixelStorei(P.UNPACK_SKIP_ROWS,jt),P.pixelStorei(P.UNPACK_SKIP_IMAGES,Ct),N===0&&D.generateMipmaps&&P.generateMipmap(P.TEXTURE_2D),ut.unbindTexture()},this.copyTextureToTexture3D=function(b,D,V=null,G=null,N=0){b.isTexture!==!0&&(Zr("WebGLRenderer: copyTextureToTexture3D function signature has changed."),V=arguments[0]||null,G=arguments[1]||null,b=arguments[2],D=arguments[3],N=arguments[4]||0);let at,mt,bt,Et,Dt,Ut,Tt,Qt,te,re=b.isCompressedTexture?b.mipmaps[N]:b.image;V!==null?(at=V.max.x-V.min.x,mt=V.max.y-V.min.y,bt=V.max.z-V.min.z,Et=V.min.x,Dt=V.min.y,Ut=V.min.z):(at=re.width,mt=re.height,bt=re.depth,Et=0,Dt=0,Ut=0),G!==null?(Tt=G.x,Qt=G.y,te=G.z):(Tt=0,Qt=0,te=0);let He=wt.convert(D.format),jt=wt.convert(D.type),Ct;if(D.isData3DTexture)M.setTexture3D(D,0),Ct=P.TEXTURE_3D;else if(D.isDataArrayTexture||D.isCompressedArrayTexture)M.setTexture2DArray(D,0),Ct=P.TEXTURE_2D_ARRAY;else{console.warn("THREE.WebGLRenderer.copyTextureToTexture3D: only supports THREE.DataTexture3D and THREE.DataTexture2DArray.");return}P.pixelStorei(P.UNPACK_FLIP_Y_WEBGL,D.flipY),P.pixelStorei(P.UNPACK_PREMULTIPLY_ALPHA_WEBGL,D.premultiplyAlpha),P.pixelStorei(P.UNPACK_ALIGNMENT,D.unpackAlignment);let Se=P.getParameter(P.UNPACK_ROW_LENGTH),Kt=P.getParameter(P.UNPACK_IMAGE_HEIGHT),rn=P.getParameter(P.UNPACK_SKIP_PIXELS),zi=P.getParameter(P.UNPACK_SKIP_ROWS),Ve=P.getParameter(P.UNPACK_SKIP_IMAGES);P.pixelStorei(P.UNPACK_ROW_LENGTH,re.width),P.pixelStorei(P.UNPACK_IMAGE_HEIGHT,re.height),P.pixelStorei(P.UNPACK_SKIP_PIXELS,Et),P.pixelStorei(P.UNPACK_SKIP_ROWS,Dt),P.pixelStorei(P.UNPACK_SKIP_IMAGES,Ut),b.isDataTexture||b.isData3DTexture?P.texSubImage3D(Ct,N,Tt,Qt,te,at,mt,bt,He,jt,re.data):D.isCompressedArrayTexture?P.compressedTexSubImage3D(Ct,N,Tt,Qt,te,at,mt,bt,He,re.data):P.texSubImage3D(Ct,N,Tt,Qt,te,at,mt,bt,He,jt,re),P.pixelStorei(P.UNPACK_ROW_LENGTH,Se),P.pixelStorei(P.UNPACK_IMAGE_HEIGHT,Kt),P.pixelStorei(P.UNPACK_SKIP_PIXELS,rn),P.pixelStorei(P.UNPACK_SKIP_ROWS,zi),P.pixelStorei(P.UNPACK_SKIP_IMAGES,Ve),N===0&&D.generateMipmaps&&P.generateMipmap(Ct),ut.unbindTexture()},this.initRenderTarget=function(b){B.get(b).__webglFramebuffer===void 0&&M.setupRenderTarget(b)},this.initTexture=function(b){b.isCubeTexture?M.setTextureCube(b,0):b.isData3DTexture?M.setTexture3D(b,0):b.isDataArrayTexture||b.isCompressedArrayTexture?M.setTexture2DArray(b,0):M.setTexture2D(b,0),ut.unbindTexture()},this.resetState=function(){I=0,C=0,E=null,ut.reset(),Ft.reset()},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}get coordinateSystem(){return Pn}get outputColorSpace(){return this._outputColorSpace}set outputColorSpace(t){this._outputColorSpace=t;let e=this.getContext();e.drawingBufferColorSpace=t===dc?"display-p3":"srgb",e.unpackColorSpace=Jt.workingColorSpace===Ao?"display-p3":"srgb"}},fo=class n{constructor(t,e=25e-5){this.isFogExp2=!0,this.name="",this.color=new Ot(t),this.density=e}clone(){return new n(this.color,this.density)}toJSON(){return{type:"FogExp2",name:this.name,color:this.color.getHex(),density:this.density}}};var po=class extends Be{constructor(){super(),this.isScene=!0,this.type="Scene",this.background=null,this.environment=null,this.fog=null,this.backgroundBlurriness=0,this.backgroundIntensity=1,this.backgroundRotation=new tn,this.environmentIntensity=1,this.environmentRotation=new tn,this.overrideMaterial=null,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}copy(t,e){return super.copy(t,e),t.background!==null&&(this.background=t.background.clone()),t.environment!==null&&(this.environment=t.environment.clone()),t.fog!==null&&(this.fog=t.fog.clone()),this.backgroundBlurriness=t.backgroundBlurriness,this.backgroundIntensity=t.backgroundIntensity,this.backgroundRotation.copy(t.backgroundRotation),this.environmentIntensity=t.environmentIntensity,this.environmentRotation.copy(t.environmentRotation),t.overrideMaterial!==null&&(this.overrideMaterial=t.overrideMaterial.clone()),this.matrixAutoUpdate=t.matrixAutoUpdate,this}toJSON(t){let e=super.toJSON(t);return this.fog!==null&&(e.object.fog=this.fog.toJSON()),this.backgroundBlurriness>0&&(e.object.backgroundBlurriness=this.backgroundBlurriness),this.backgroundIntensity!==1&&(e.object.backgroundIntensity=this.backgroundIntensity),e.object.backgroundRotation=this.backgroundRotation.toArray(),this.environmentIntensity!==1&&(e.object.environmentIntensity=this.environmentIntensity),e.object.environmentRotation=this.environmentRotation.toArray(),e}};var Wl=class extends Ce{constructor(t=null,e=1,i=1,s,r,o,a,l,h=be,f=be,d,c){super(null,o,a,l,h,f,s,r,d,c),this.isDataTexture=!0,this.image={data:t,width:e,height:i},this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}};var Un=class extends qe{constructor(t,e,i,s=1){super(t,e,i),this.isInstancedBufferAttribute=!0,this.meshPerAttribute=s}copy(t){return super.copy(t),this.meshPerAttribute=t.meshPerAttribute,this}toJSON(){let t=super.toJSON();return t.meshPerAttribute=this.meshPerAttribute,t.isInstancedBufferAttribute=!0,t}},ts=new $t,eu=new $t,Hr=[],nu=new Dn,Nx=new $t,qs=new Ue,Ys=new _i,ir=class extends Ue{constructor(t,e,i){super(t,e),this.isInstancedMesh=!0,this.instanceMatrix=new Un(new Float32Array(i*16),16),this.instanceColor=null,this.morphTexture=null,this.count=i,this.boundingBox=null,this.boundingSphere=null;for(let s=0;s<i;s++)this.setMatrixAt(s,Nx)}computeBoundingBox(){let t=this.geometry,e=this.count;this.boundingBox===null&&(this.boundingBox=new Dn),t.boundingBox===null&&t.computeBoundingBox(),this.boundingBox.makeEmpty();for(let i=0;i<e;i++)this.getMatrixAt(i,ts),nu.copy(t.boundingBox).applyMatrix4(ts),this.boundingBox.union(nu)}computeBoundingSphere(){let t=this.geometry,e=this.count;this.boundingSphere===null&&(this.boundingSphere=new _i),t.boundingSphere===null&&t.computeBoundingSphere(),this.boundingSphere.makeEmpty();for(let i=0;i<e;i++)this.getMatrixAt(i,ts),Ys.copy(t.boundingSphere).applyMatrix4(ts),this.boundingSphere.union(Ys)}copy(t,e){return super.copy(t,e),this.instanceMatrix.copy(t.instanceMatrix),t.morphTexture!==null&&(this.morphTexture=t.morphTexture.clone()),t.instanceColor!==null&&(this.instanceColor=t.instanceColor.clone()),this.count=t.count,t.boundingBox!==null&&(this.boundingBox=t.boundingBox.clone()),t.boundingSphere!==null&&(this.boundingSphere=t.boundingSphere.clone()),this}getColorAt(t,e){e.fromArray(this.instanceColor.array,t*3)}getMatrixAt(t,e){e.fromArray(this.instanceMatrix.array,t*16)}getMorphAt(t,e){let i=e.morphTargetInfluences,s=this.morphTexture.source.data.data,r=i.length+1,o=t*r+1;for(let a=0;a<i.length;a++)i[a]=s[o+a]}raycast(t,e){let i=this.matrixWorld,s=this.count;if(qs.geometry=this.geometry,qs.material=this.material,qs.material!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),Ys.copy(this.boundingSphere),Ys.applyMatrix4(i),t.ray.intersectsSphere(Ys)!==!1))for(let r=0;r<s;r++){this.getMatrixAt(r,ts),eu.multiplyMatrices(i,ts),qs.matrixWorld=eu,qs.raycast(t,Hr);for(let o=0,a=Hr.length;o<a;o++){let l=Hr[o];l.instanceId=r,l.object=this,e.push(l)}Hr.length=0}}setColorAt(t,e){this.instanceColor===null&&(this.instanceColor=new Un(new Float32Array(this.instanceMatrix.count*3).fill(1),3)),e.toArray(this.instanceColor.array,t*3)}setMatrixAt(t,e){e.toArray(this.instanceMatrix.array,t*16)}setMorphAt(t,e){let i=e.morphTargetInfluences,s=i.length+1;this.morphTexture===null&&(this.morphTexture=new Wl(new Float32Array(s*this.count),s,this.count,lc,xn));let r=this.morphTexture.source.data.data,o=0;for(let h=0;h<i.length;h++)o+=i[h];let a=this.geometry.morphTargetsRelative?1:1-o,l=s*t;r[l]=a,r.set(i,l+1)}updateMorphTargets(){}dispose(){return this.dispatchEvent({type:"dispose"}),this.morphTexture!==null&&(this.morphTexture.dispose(),this.morphTexture=null),this}};var mo=class n extends un{constructor(t=1,e=1,i=1,s=32,r=1,o=!1,a=0,l=Math.PI*2){super(),this.type="CylinderGeometry",this.parameters={radiusTop:t,radiusBottom:e,height:i,radialSegments:s,heightSegments:r,openEnded:o,thetaStart:a,thetaLength:l};let h=this;s=Math.floor(s),r=Math.floor(r);let f=[],d=[],c=[],u=[],g=0,y=[],p=i/2,m=0;w(),o===!1&&(t>0&&_(!0),e>0&&_(!1)),this.setIndex(f),this.setAttribute("position",new ve(d,3)),this.setAttribute("normal",new ve(c,3)),this.setAttribute("uv",new ve(u,2));function w(){let A=new U,I=new U,C=0,E=(e-t)/i;for(let T=0;T<=r;T++){let H=[],x=T/r,S=x*(e-t)+t;for(let O=0;O<=s;O++){let W=O/s,q=W*l+a,et=Math.sin(q),F=Math.cos(q);I.x=S*et,I.y=-x*i+p,I.z=S*F,d.push(I.x,I.y,I.z),A.set(et,E,F).normalize(),c.push(A.x,A.y,A.z),u.push(W,1-x),H.push(g++)}y.push(H)}for(let T=0;T<s;T++)for(let H=0;H<r;H++){let x=y[H][T],S=y[H+1][T],O=y[H+1][T+1],W=y[H][T+1];t>0&&(f.push(x,S,W),C+=3),e>0&&(f.push(S,O,W),C+=3)}h.addGroup(m,C,0),m+=C}function _(A){let I=g,C=new yt,E=new U,T=0,H=A===!0?t:e,x=A===!0?1:-1;for(let O=1;O<=s;O++)d.push(0,p*x,0),c.push(0,x,0),u.push(.5,.5),g++;let S=g;for(let O=0;O<=s;O++){let q=O/s*l+a,et=Math.cos(q),F=Math.sin(q);E.x=H*F,E.y=p*x,E.z=H*et,d.push(E.x,E.y,E.z),c.push(0,x,0),C.x=et*.5+.5,C.y=F*.5*x+.5,u.push(C.x,C.y),g++}for(let O=0;O<s;O++){let W=I+O,q=S+O;A===!0?f.push(q,q+1,W):f.push(q+1,q,W),T+=3}h.addGroup(m,T,A===!0?1:2),m+=T}}copy(t){return super.copy(t),this.parameters=Object.assign({},t.parameters),this}static fromJSON(t){return new n(t.radiusTop,t.radiusBottom,t.height,t.radialSegments,t.heightSegments,t.openEnded,t.thetaStart,t.thetaLength)}};var Xl=class n extends un{constructor(t=[],e=[],i=1,s=0){super(),this.type="PolyhedronGeometry",this.parameters={vertices:t,indices:e,radius:i,detail:s};let r=[],o=[];a(s),h(i),f(),this.setAttribute("position",new ve(r,3)),this.setAttribute("normal",new ve(r.slice(),3)),this.setAttribute("uv",new ve(o,2)),s===0?this.computeVertexNormals():this.normalizeNormals();function a(w){let _=new U,A=new U,I=new U;for(let C=0;C<e.length;C+=3)u(e[C+0],_),u(e[C+1],A),u(e[C+2],I),l(_,A,I,w)}function l(w,_,A,I){let C=I+1,E=[];for(let T=0;T<=C;T++){E[T]=[];let H=w.clone().lerp(A,T/C),x=_.clone().lerp(A,T/C),S=C-T;for(let O=0;O<=S;O++)O===0&&T===C?E[T][O]=H:E[T][O]=H.clone().lerp(x,O/S)}for(let T=0;T<C;T++)for(let H=0;H<2*(C-T)-1;H++){let x=Math.floor(H/2);H%2===0?(c(E[T][x+1]),c(E[T+1][x]),c(E[T][x])):(c(E[T][x+1]),c(E[T+1][x+1]),c(E[T+1][x]))}}function h(w){let _=new U;for(let A=0;A<r.length;A+=3)_.x=r[A+0],_.y=r[A+1],_.z=r[A+2],_.normalize().multiplyScalar(w),r[A+0]=_.x,r[A+1]=_.y,r[A+2]=_.z}function f(){let w=new U;for(let _=0;_<r.length;_+=3){w.x=r[_+0],w.y=r[_+1],w.z=r[_+2];let A=p(w)/2/Math.PI+.5,I=m(w)/Math.PI+.5;o.push(A,1-I)}g(),d()}function d(){for(let w=0;w<o.length;w+=6){let _=o[w+0],A=o[w+2],I=o[w+4],C=Math.max(_,A,I),E=Math.min(_,A,I);C>.9&&E<.1&&(_<.2&&(o[w+0]+=1),A<.2&&(o[w+2]+=1),I<.2&&(o[w+4]+=1))}}function c(w){r.push(w.x,w.y,w.z)}function u(w,_){let A=w*3;_.x=t[A+0],_.y=t[A+1],_.z=t[A+2]}function g(){let w=new U,_=new U,A=new U,I=new U,C=new yt,E=new yt,T=new yt;for(let H=0,x=0;H<r.length;H+=9,x+=6){w.set(r[H+0],r[H+1],r[H+2]),_.set(r[H+3],r[H+4],r[H+5]),A.set(r[H+6],r[H+7],r[H+8]),C.set(o[x+0],o[x+1]),E.set(o[x+2],o[x+3]),T.set(o[x+4],o[x+5]),I.copy(w).add(_).add(A).divideScalar(3);let S=p(I);y(C,x+0,w,S),y(E,x+2,_,S),y(T,x+4,A,S)}}function y(w,_,A,I){I<0&&w.x===1&&(o[_]=w.x-1),A.x===0&&A.z===0&&(o[_]=I/2/Math.PI+.5)}function p(w){return Math.atan2(w.z,-w.x)}function m(w){return Math.atan2(-w.y,Math.sqrt(w.x*w.x+w.z*w.z))}}copy(t){return super.copy(t),this.parameters=Object.assign({},t.parameters),this}static fromJSON(t){return new n(t.vertices,t.indices,t.radius,t.details)}};var go=class n extends Xl{constructor(t=1,e=0){let i=(1+Math.sqrt(5))/2,s=[-1,i,0,1,i,0,-1,-i,0,1,-i,0,0,-1,i,0,1,i,0,-1,-i,0,1,-i,i,0,-1,i,0,1,-i,0,-1,-i,0,1],r=[0,11,5,0,5,1,0,1,7,0,7,10,0,10,11,1,5,9,5,11,4,11,10,2,10,7,6,7,1,8,3,9,4,3,4,2,3,2,6,3,6,8,3,8,9,4,9,5,2,4,11,6,2,10,8,6,7,9,8,1];super(s,r,t,e),this.type="IcosahedronGeometry",this.parameters={radius:t,detail:e}}static fromJSON(t){return new n(t.radius,t.detail)}};var xo=class extends yi{constructor(t){super(),this.isMeshStandardMaterial=!0,this.defines={STANDARD:""},this.type="MeshStandardMaterial",this.color=new Ot(16777215),this.roughness=1,this.metalness=0,this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new Ot(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=yu,this.normalScale=new yt(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.roughnessMap=null,this.metalnessMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new tn,this.envMapIntensity=1,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(t)}copy(t){return super.copy(t),this.defines={STANDARD:""},this.color.copy(t.color),this.roughness=t.roughness,this.metalness=t.metalness,this.map=t.map,this.lightMap=t.lightMap,this.lightMapIntensity=t.lightMapIntensity,this.aoMap=t.aoMap,this.aoMapIntensity=t.aoMapIntensity,this.emissive.copy(t.emissive),this.emissiveMap=t.emissiveMap,this.emissiveIntensity=t.emissiveIntensity,this.bumpMap=t.bumpMap,this.bumpScale=t.bumpScale,this.normalMap=t.normalMap,this.normalMapType=t.normalMapType,this.normalScale.copy(t.normalScale),this.displacementMap=t.displacementMap,this.displacementScale=t.displacementScale,this.displacementBias=t.displacementBias,this.roughnessMap=t.roughnessMap,this.metalnessMap=t.metalnessMap,this.alphaMap=t.alphaMap,this.envMap=t.envMap,this.envMapRotation.copy(t.envMapRotation),this.envMapIntensity=t.envMapIntensity,this.wireframe=t.wireframe,this.wireframeLinewidth=t.wireframeLinewidth,this.wireframeLinecap=t.wireframeLinecap,this.wireframeLinejoin=t.wireframeLinejoin,this.flatShading=t.flatShading,this.fog=t.fog,this}};function Vr(n,t,e){return!n||!e&&n.constructor===t?n:typeof t.BYTES_PER_ELEMENT=="number"?new t(n):Array.prototype.slice.call(n)}function Ox(n){return ArrayBuffer.isView(n)&&!(n instanceof DataView)}var gs=class{constructor(t,e,i,s){this.parameterPositions=t,this._cachedIndex=0,this.resultBuffer=s!==void 0?s:new e.constructor(i),this.sampleValues=e,this.valueSize=i,this.settings=null,this.DefaultSettings_={}}evaluate(t){let e=this.parameterPositions,i=this._cachedIndex,s=e[i],r=e[i-1];n:{t:{let o;e:{i:if(!(t<s)){for(let a=i+2;;){if(s===void 0){if(t<r)break i;return i=e.length,this._cachedIndex=i,this.copySampleValue_(i-1)}if(i===a)break;if(r=s,s=e[++i],t<s)break t}o=e.length;break e}if(!(t>=r)){let a=e[1];t<a&&(i=2,r=a);for(let l=i-2;;){if(r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(i===l)break;if(s=r,r=e[--i-1],t>=r)break t}o=i,i=0;break e}break n}for(;i<o;){let a=i+o>>>1;t<e[a]?o=a:i=a+1}if(s=e[i],r=e[i-1],r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(s===void 0)return i=e.length,this._cachedIndex=i,this.copySampleValue_(i-1)}this._cachedIndex=i,this.intervalChanged_(i,r,s)}return this.interpolate_(i,r,t,s)}getSettings_(){return this.settings||this.DefaultSettings_}copySampleValue_(t){let e=this.resultBuffer,i=this.sampleValues,s=this.valueSize,r=t*s;for(let o=0;o!==s;++o)e[o]=i[r+o];return e}interpolate_(){throw new Error("call to abstract method")}intervalChanged_(){}},ql=class extends gs{constructor(t,e,i,s){super(t,e,i,s),this._weightPrev=-0,this._offsetPrev=-0,this._weightNext=-0,this._offsetNext=-0,this.DefaultSettings_={endingStart:lh,endingEnd:lh}}intervalChanged_(t,e,i){let s=this.parameterPositions,r=t-2,o=t+1,a=s[r],l=s[o];if(a===void 0)switch(this.getSettings_().endingStart){case ch:r=t,a=2*e-i;break;case hh:r=s.length-2,a=e+s[r]-s[r+1];break;default:r=t,a=i}if(l===void 0)switch(this.getSettings_().endingEnd){case ch:o=t,l=2*i-e;break;case hh:o=1,l=i+s[1]-s[0];break;default:o=t-1,l=e}let h=(i-e)*.5,f=this.valueSize;this._weightPrev=h/(e-a),this._weightNext=h/(l-i),this._offsetPrev=r*f,this._offsetNext=o*f}interpolate_(t,e,i,s){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=t*a,h=l-a,f=this._offsetPrev,d=this._offsetNext,c=this._weightPrev,u=this._weightNext,g=(i-e)/(s-e),y=g*g,p=y*g,m=-c*p+2*c*y-c*g,w=(1+c)*p+(-1.5-2*c)*y+(-.5+c)*g+1,_=(-1-u)*p+(1.5+u)*y+.5*g,A=u*p-u*y;for(let I=0;I!==a;++I)r[I]=m*o[f+I]+w*o[h+I]+_*o[l+I]+A*o[d+I];return r}},Yl=class extends gs{constructor(t,e,i,s){super(t,e,i,s)}interpolate_(t,e,i,s){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=t*a,h=l-a,f=(i-e)/(s-e),d=1-f;for(let c=0;c!==a;++c)r[c]=o[h+c]*d+o[l+c]*f;return r}},Zl=class extends gs{constructor(t,e,i,s){super(t,e,i,s)}interpolate_(t){return this.copySampleValue_(t-1)}},dn=class{constructor(t,e,i,s){if(t===void 0)throw new Error("THREE.KeyframeTrack: track name is undefined");if(e===void 0||e.length===0)throw new Error("THREE.KeyframeTrack: no keyframes in track named "+t);this.name=t,this.times=Vr(e,this.TimeBufferType),this.values=Vr(i,this.ValueBufferType),this.setInterpolation(s||this.DefaultInterpolation)}static toJSON(t){let e=t.constructor,i;if(e.toJSON!==this.toJSON)i=e.toJSON(t);else{i={name:t.name,times:Vr(t.times,Array),values:Vr(t.values,Array)};let s=t.getInterpolation();s!==t.DefaultInterpolation&&(i.interpolation=s)}return i.type=t.ValueTypeName,i}InterpolantFactoryMethodDiscrete(t){return new Zl(this.times,this.values,this.getValueSize(),t)}InterpolantFactoryMethodLinear(t){return new Yl(this.times,this.values,this.getValueSize(),t)}InterpolantFactoryMethodSmooth(t){return new ql(this.times,this.values,this.getValueSize(),t)}setInterpolation(t){let e;switch(t){case jr:e=this.InterpolantFactoryMethodDiscrete;break;case Tl:e=this.InterpolantFactoryMethodLinear;break;case fa:e=this.InterpolantFactoryMethodSmooth;break}if(e===void 0){let i="unsupported interpolation for "+this.ValueTypeName+" keyframe track named "+this.name;if(this.createInterpolant===void 0)if(t!==this.DefaultInterpolation)this.setInterpolation(this.DefaultInterpolation);else throw new Error(i);return console.warn("THREE.KeyframeTrack:",i),this}return this.createInterpolant=e,this}getInterpolation(){switch(this.createInterpolant){case this.InterpolantFactoryMethodDiscrete:return jr;case this.InterpolantFactoryMethodLinear:return Tl;case this.InterpolantFactoryMethodSmooth:return fa}}getValueSize(){return this.values.length/this.times.length}shift(t){if(t!==0){let e=this.times;for(let i=0,s=e.length;i!==s;++i)e[i]+=t}return this}scale(t){if(t!==1){let e=this.times;for(let i=0,s=e.length;i!==s;++i)e[i]*=t}return this}trim(t,e){let i=this.times,s=i.length,r=0,o=s-1;for(;r!==s&&i[r]<t;)++r;for(;o!==-1&&i[o]>e;)--o;if(++o,r!==0||o!==s){r>=o&&(o=Math.max(o,1),r=o-1);let a=this.getValueSize();this.times=i.slice(r,o),this.values=this.values.slice(r*a,o*a)}return this}validate(){let t=!0,e=this.getValueSize();e-Math.floor(e)!==0&&(console.error("THREE.KeyframeTrack: Invalid value size in track.",this),t=!1);let i=this.times,s=this.values,r=i.length;r===0&&(console.error("THREE.KeyframeTrack: Track is empty.",this),t=!1);let o=null;for(let a=0;a!==r;a++){let l=i[a];if(typeof l=="number"&&isNaN(l)){console.error("THREE.KeyframeTrack: Time is not a valid number.",this,a,l),t=!1;break}if(o!==null&&o>l){console.error("THREE.KeyframeTrack: Out of order keys.",this,a,l,o),t=!1;break}o=l}if(s!==void 0&&Ox(s))for(let a=0,l=s.length;a!==l;++a){let h=s[a];if(isNaN(h)){console.error("THREE.KeyframeTrack: Value is not a valid number.",this,a,h),t=!1;break}}return t}optimize(){let t=this.times.slice(),e=this.values.slice(),i=this.getValueSize(),s=this.getInterpolation()===fa,r=t.length-1,o=1;for(let a=1;a<r;++a){let l=!1,h=t[a],f=t[a+1];if(h!==f&&(a!==1||h!==t[0]))if(s)l=!0;else{let d=a*i,c=d-i,u=d+i;for(let g=0;g!==i;++g){let y=e[d+g];if(y!==e[c+g]||y!==e[u+g]){l=!0;break}}}if(l){if(a!==o){t[o]=t[a];let d=a*i,c=o*i;for(let u=0;u!==i;++u)e[c+u]=e[d+u]}++o}}if(r>0){t[o]=t[r];for(let a=r*i,l=o*i,h=0;h!==i;++h)e[l+h]=e[a+h];++o}return o!==t.length?(this.times=t.slice(0,o),this.values=e.slice(0,o*i)):(this.times=t,this.values=e),this}clone(){let t=this.times.slice(),e=this.values.slice(),i=this.constructor,s=new i(this.name,t,e);return s.createInterpolant=this.createInterpolant,s}};dn.prototype.TimeBufferType=Float32Array;dn.prototype.ValueBufferType=Float32Array;dn.prototype.DefaultInterpolation=Tl;var Mi=class extends dn{constructor(t,e,i){super(t,e,i)}};Mi.prototype.ValueTypeName="bool";Mi.prototype.ValueBufferType=Array;Mi.prototype.DefaultInterpolation=jr;Mi.prototype.InterpolantFactoryMethodLinear=void 0;Mi.prototype.InterpolantFactoryMethodSmooth=void 0;var jl=class extends dn{};jl.prototype.ValueTypeName="color";var Kl=class extends dn{};Kl.prototype.ValueTypeName="number";var Jl=class extends gs{constructor(t,e,i,s){super(t,e,i,s)}interpolate_(t,e,i,s){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=(i-e)/(s-e),h=t*a;for(let f=h+a;h!==f;h+=4)$e.slerpFlat(r,0,o,h-a,o,h,l);return r}},vo=class extends dn{InterpolantFactoryMethodLinear(t){return new Jl(this.times,this.values,this.getValueSize(),t)}};vo.prototype.ValueTypeName="quaternion";vo.prototype.InterpolantFactoryMethodSmooth=void 0;var Si=class extends dn{constructor(t,e,i){super(t,e,i)}};Si.prototype.ValueTypeName="string";Si.prototype.ValueBufferType=Array;Si.prototype.DefaultInterpolation=jr;Si.prototype.InterpolantFactoryMethodLinear=void 0;Si.prototype.InterpolantFactoryMethodSmooth=void 0;var Ql=class extends dn{};Ql.prototype.ValueTypeName="vector";var $l=class{constructor(t,e,i){let s=this,r=!1,o=0,a=0,l,h=[];this.onStart=void 0,this.onLoad=t,this.onProgress=e,this.onError=i,this.itemStart=function(f){a++,r===!1&&s.onStart!==void 0&&s.onStart(f,o,a),r=!0},this.itemEnd=function(f){o++,s.onProgress!==void 0&&s.onProgress(f,o,a),o===a&&(r=!1,s.onLoad!==void 0&&s.onLoad())},this.itemError=function(f){s.onError!==void 0&&s.onError(f)},this.resolveURL=function(f){return l?l(f):f},this.setURLModifier=function(f){return l=f,this},this.addHandler=function(f,d){return h.push(f,d),this},this.removeHandler=function(f){let d=h.indexOf(f);return d!==-1&&h.splice(d,2),this},this.getHandler=function(f){for(let d=0,c=h.length;d<c;d+=2){let u=h[d],g=h[d+1];if(u.global&&(u.lastIndex=0),u.test(f))return g}return null}}},Fx=new $l,tc=class{constructor(t){this.manager=t!==void 0?t:Fx,this.crossOrigin="anonymous",this.withCredentials=!1,this.path="",this.resourcePath="",this.requestHeader={}}load(){}loadAsync(t,e){let i=this;return new Promise(function(s,r){i.load(t,s,e,r)})}parse(){}setCrossOrigin(t){return this.crossOrigin=t,this}setWithCredentials(t){return this.withCredentials=t,this}setPath(t){return this.path=t,this}setResourcePath(t){return this.resourcePath=t,this}setRequestHeader(t){return this.requestHeader=t,this}};tc.DEFAULT_MATERIAL_NAME="__DEFAULT";var _o=class extends Be{constructor(t,e=1){super(),this.isLight=!0,this.type="Light",this.color=new Ot(t),this.intensity=e}dispose(){}copy(t,e){return super.copy(t,e),this.color.copy(t.color),this.intensity=t.intensity,this}toJSON(t){let e=super.toJSON(t);return e.object.color=this.color.getHex(),e.object.intensity=this.intensity,this.groundColor!==void 0&&(e.object.groundColor=this.groundColor.getHex()),this.distance!==void 0&&(e.object.distance=this.distance),this.angle!==void 0&&(e.object.angle=this.angle),this.decay!==void 0&&(e.object.decay=this.decay),this.penumbra!==void 0&&(e.object.penumbra=this.penumbra),this.shadow!==void 0&&(e.object.shadow=this.shadow.toJSON()),this.target!==void 0&&(e.object.target=this.target.uuid),e}};var Ha=new $t,iu=new U,su=new U,ec=class{constructor(t){this.camera=t,this.intensity=1,this.bias=0,this.normalBias=0,this.radius=1,this.blurSamples=8,this.mapSize=new yt(512,512),this.map=null,this.mapPass=null,this.matrix=new $t,this.autoUpdate=!0,this.needsUpdate=!1,this._frustum=new nr,this._frameExtents=new yt(1,1),this._viewportCount=1,this._viewports=[new oe(0,0,1,1)]}getViewportCount(){return this._viewportCount}getFrustum(){return this._frustum}updateMatrices(t){let e=this.camera,i=this.matrix;iu.setFromMatrixPosition(t.matrixWorld),e.position.copy(iu),su.setFromMatrixPosition(t.target.matrixWorld),e.lookAt(su),e.updateMatrixWorld(),Ha.multiplyMatrices(e.projectionMatrix,e.matrixWorldInverse),this._frustum.setFromProjectionMatrix(Ha),i.set(.5,0,0,.5,0,.5,0,.5,0,0,.5,.5,0,0,0,1),i.multiply(Ha)}getViewport(t){return this._viewports[t]}getFrameExtents(){return this._frameExtents}dispose(){this.map&&this.map.dispose(),this.mapPass&&this.mapPass.dispose()}copy(t){return this.camera=t.camera.clone(),this.intensity=t.intensity,this.bias=t.bias,this.radius=t.radius,this.mapSize.copy(t.mapSize),this}clone(){return new this.constructor().copy(this)}toJSON(){let t={};return this.intensity!==1&&(t.intensity=this.intensity),this.bias!==0&&(t.bias=this.bias),this.normalBias!==0&&(t.normalBias=this.normalBias),this.radius!==1&&(t.radius=this.radius),(this.mapSize.x!==512||this.mapSize.y!==512)&&(t.mapSize=this.mapSize.toArray()),t.camera=this.camera.toJSON(!1).object,delete t.camera.matrix,t}};var nc=class extends ec{constructor(){super(new ms(-5,5,5,-5,.5,500)),this.isDirectionalLightShadow=!0}},sr=class extends _o{constructor(t,e){super(t,e),this.isDirectionalLight=!0,this.type="DirectionalLight",this.position.copy(Be.DEFAULT_UP),this.updateMatrix(),this.target=new Be,this.shadow=new nc}dispose(){this.shadow.dispose()}copy(t){return super.copy(t),this.target=t.target.clone(),this.shadow=t.shadow.clone(),this}},yo=class extends _o{constructor(t,e){super(t,e),this.isAmbientLight=!0,this.type="AmbientLight"}};var Mo=class{constructor(t=!0){this.autoStart=t,this.startTime=0,this.oldTime=0,this.elapsedTime=0,this.running=!1}start(){this.startTime=ru(),this.oldTime=this.startTime,this.elapsedTime=0,this.running=!0}stop(){this.getElapsedTime(),this.running=!1,this.autoStart=!1}getElapsedTime(){return this.getDelta(),this.elapsedTime}getDelta(){let t=0;if(this.autoStart&&!this.running)return this.start(),0;if(this.running){let e=ru();t=(e-this.oldTime)/1e3,this.oldTime=e,this.elapsedTime+=t}return t}};function ru(){return performance.now()}var gc="\\[\\]\\.:\\/",Bx=new RegExp("["+gc+"]","g"),xc="[^"+gc+"]",zx="[^"+gc.replace("\\.","")+"]",kx=/((?:WC+[\/:])*)/.source.replace("WC",xc),Hx=/(WCOD+)?/.source.replace("WCOD",zx),Vx=/(?:\.(WC+)(?:\[(.+)\])?)?/.source.replace("WC",xc),Gx=/\.(WC+)(?:\[(.+)\])?/.source.replace("WC",xc),Wx=new RegExp("^"+kx+Hx+Vx+Gx+"$"),Xx=["material","materials","bones","map"],ic=class{constructor(t,e,i){let s=i||ne.parseTrackName(e);this._targetGroup=t,this._bindings=t.subscribe_(e,s)}getValue(t,e){this.bind();let i=this._targetGroup.nCachedObjects_,s=this._bindings[i];s!==void 0&&s.getValue(t,e)}setValue(t,e){let i=this._bindings;for(let s=this._targetGroup.nCachedObjects_,r=i.length;s!==r;++s)i[s].setValue(t,e)}bind(){let t=this._bindings;for(let e=this._targetGroup.nCachedObjects_,i=t.length;e!==i;++e)t[e].bind()}unbind(){let t=this._bindings;for(let e=this._targetGroup.nCachedObjects_,i=t.length;e!==i;++e)t[e].unbind()}},ne=class n{constructor(t,e,i){this.path=e,this.parsedPath=i||n.parseTrackName(e),this.node=n.findNode(t,this.parsedPath.nodeName),this.rootNode=t,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}static create(t,e,i){return t&&t.isAnimationObjectGroup?new n.Composite(t,e,i):new n(t,e,i)}static sanitizeNodeName(t){return t.replace(/\s/g,"_").replace(Bx,"")}static parseTrackName(t){let e=Wx.exec(t);if(e===null)throw new Error("PropertyBinding: Cannot parse trackName: "+t);let i={nodeName:e[2],objectName:e[3],objectIndex:e[4],propertyName:e[5],propertyIndex:e[6]},s=i.nodeName&&i.nodeName.lastIndexOf(".");if(s!==void 0&&s!==-1){let r=i.nodeName.substring(s+1);Xx.indexOf(r)!==-1&&(i.nodeName=i.nodeName.substring(0,s),i.objectName=r)}if(i.propertyName===null||i.propertyName.length===0)throw new Error("PropertyBinding: can not parse propertyName from trackName: "+t);return i}static findNode(t,e){if(e===void 0||e===""||e==="."||e===-1||e===t.name||e===t.uuid)return t;if(t.skeleton){let i=t.skeleton.getBoneByName(e);if(i!==void 0)return i}if(t.children){let i=function(r){for(let o=0;o<r.length;o++){let a=r[o];if(a.name===e||a.uuid===e)return a;let l=i(a.children);if(l)return l}return null},s=i(t.children);if(s)return s}return null}_getValue_unavailable(){}_setValue_unavailable(){}_getValue_direct(t,e){t[e]=this.targetObject[this.propertyName]}_getValue_array(t,e){let i=this.resolvedProperty;for(let s=0,r=i.length;s!==r;++s)t[e++]=i[s]}_getValue_arrayElement(t,e){t[e]=this.resolvedProperty[this.propertyIndex]}_getValue_toArray(t,e){this.resolvedProperty.toArray(t,e)}_setValue_direct(t,e){this.targetObject[this.propertyName]=t[e]}_setValue_direct_setNeedsUpdate(t,e){this.targetObject[this.propertyName]=t[e],this.targetObject.needsUpdate=!0}_setValue_direct_setMatrixWorldNeedsUpdate(t,e){this.targetObject[this.propertyName]=t[e],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_array(t,e){let i=this.resolvedProperty;for(let s=0,r=i.length;s!==r;++s)i[s]=t[e++]}_setValue_array_setNeedsUpdate(t,e){let i=this.resolvedProperty;for(let s=0,r=i.length;s!==r;++s)i[s]=t[e++];this.targetObject.needsUpdate=!0}_setValue_array_setMatrixWorldNeedsUpdate(t,e){let i=this.resolvedProperty;for(let s=0,r=i.length;s!==r;++s)i[s]=t[e++];this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_arrayElement(t,e){this.resolvedProperty[this.propertyIndex]=t[e]}_setValue_arrayElement_setNeedsUpdate(t,e){this.resolvedProperty[this.propertyIndex]=t[e],this.targetObject.needsUpdate=!0}_setValue_arrayElement_setMatrixWorldNeedsUpdate(t,e){this.resolvedProperty[this.propertyIndex]=t[e],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_fromArray(t,e){this.resolvedProperty.fromArray(t,e)}_setValue_fromArray_setNeedsUpdate(t,e){this.resolvedProperty.fromArray(t,e),this.targetObject.needsUpdate=!0}_setValue_fromArray_setMatrixWorldNeedsUpdate(t,e){this.resolvedProperty.fromArray(t,e),this.targetObject.matrixWorldNeedsUpdate=!0}_getValue_unbound(t,e){this.bind(),this.getValue(t,e)}_setValue_unbound(t,e){this.bind(),this.setValue(t,e)}bind(){let t=this.node,e=this.parsedPath,i=e.objectName,s=e.propertyName,r=e.propertyIndex;if(t||(t=n.findNode(this.rootNode,e.nodeName),this.node=t),this.getValue=this._getValue_unavailable,this.setValue=this._setValue_unavailable,!t){console.warn("THREE.PropertyBinding: No target node found for track: "+this.path+".");return}if(i){let h=e.objectIndex;switch(i){case"materials":if(!t.material){console.error("THREE.PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!t.material.materials){console.error("THREE.PropertyBinding: Can not bind to material.materials as node.material does not have a materials array.",this);return}t=t.material.materials;break;case"bones":if(!t.skeleton){console.error("THREE.PropertyBinding: Can not bind to bones as node does not have a skeleton.",this);return}t=t.skeleton.bones;for(let f=0;f<t.length;f++)if(t[f].name===h){h=f;break}break;case"map":if("map"in t){t=t.map;break}if(!t.material){console.error("THREE.PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!t.material.map){console.error("THREE.PropertyBinding: Can not bind to material.map as node.material does not have a map.",this);return}t=t.material.map;break;default:if(t[i]===void 0){console.error("THREE.PropertyBinding: Can not bind to objectName of node undefined.",this);return}t=t[i]}if(h!==void 0){if(t[h]===void 0){console.error("THREE.PropertyBinding: Trying to bind to objectIndex of objectName, but is undefined.",this,t);return}t=t[h]}}let o=t[s];if(o===void 0){let h=e.nodeName;console.error("THREE.PropertyBinding: Trying to update property for track: "+h+"."+s+" but it wasn't found.",t);return}let a=this.Versioning.None;this.targetObject=t,t.needsUpdate!==void 0?a=this.Versioning.NeedsUpdate:t.matrixWorldNeedsUpdate!==void 0&&(a=this.Versioning.MatrixWorldNeedsUpdate);let l=this.BindingType.Direct;if(r!==void 0){if(s==="morphTargetInfluences"){if(!t.geometry){console.error("THREE.PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.",this);return}if(!t.geometry.morphAttributes){console.error("THREE.PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.morphAttributes.",this);return}t.morphTargetDictionary[r]!==void 0&&(r=t.morphTargetDictionary[r])}l=this.BindingType.ArrayElement,this.resolvedProperty=o,this.propertyIndex=r}else o.fromArray!==void 0&&o.toArray!==void 0?(l=this.BindingType.HasFromToArray,this.resolvedProperty=o):Array.isArray(o)?(l=this.BindingType.EntireArray,this.resolvedProperty=o):this.propertyName=s;this.getValue=this.GetterByBindingType[l],this.setValue=this.SetterByBindingTypeAndVersioning[l][a]}unbind(){this.node=null,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}};ne.Composite=ic;ne.prototype.BindingType={Direct:0,EntireArray:1,ArrayElement:2,HasFromToArray:3};ne.prototype.Versioning={None:0,NeedsUpdate:1,MatrixWorldNeedsUpdate:2};ne.prototype.GetterByBindingType=[ne.prototype._getValue_direct,ne.prototype._getValue_array,ne.prototype._getValue_arrayElement,ne.prototype._getValue_toArray];ne.prototype.SetterByBindingTypeAndVersioning=[[ne.prototype._setValue_direct,ne.prototype._setValue_direct_setNeedsUpdate,ne.prototype._setValue_direct_setMatrixWorldNeedsUpdate],[ne.prototype._setValue_array,ne.prototype._setValue_array_setNeedsUpdate,ne.prototype._setValue_array_setMatrixWorldNeedsUpdate],[ne.prototype._setValue_arrayElement,ne.prototype._setValue_arrayElement_setNeedsUpdate,ne.prototype._setValue_arrayElement_setMatrixWorldNeedsUpdate],[ne.prototype._setValue_fromArray,ne.prototype._setValue_fromArray_setNeedsUpdate,ne.prototype._setValue_fromArray_setMatrixWorldNeedsUpdate]];var a_=new Float32Array(1);var ou=new $t,So=class{constructor(t,e,i=0,s=1/0){this.ray=new io(t,e),this.near=i,this.far=s,this.camera=null,this.layers=new tr,this.params={Mesh:{},Line:{threshold:1},LOD:{},Points:{threshold:1},Sprite:{}}}set(t,e){this.ray.set(t,e)}setFromCamera(t,e){e.isPerspectiveCamera?(this.ray.origin.setFromMatrixPosition(e.matrixWorld),this.ray.direction.set(t.x,t.y,.5).unproject(e).sub(this.ray.origin).normalize(),this.camera=e):e.isOrthographicCamera?(this.ray.origin.set(t.x,t.y,(e.near+e.far)/(e.near-e.far)).unproject(e),this.ray.direction.set(0,0,-1).transformDirection(e.matrixWorld),this.camera=e):console.error("THREE.Raycaster: Unsupported camera type: "+e.type)}setFromXRController(t){return ou.identity().extractRotation(t.matrixWorld),this.ray.origin.setFromMatrixPosition(t.matrixWorld),this.ray.direction.set(0,0,-1).applyMatrix4(ou),this}intersectObject(t,e=!0,i=[]){return sc(t,this,i,e),i.sort(au),i}intersectObjects(t,e=!0,i=[]){for(let s=0,r=t.length;s<r;s++)sc(t[s],this,i,e);return i.sort(au),i}};function au(n,t){return n.distance-t.distance}function sc(n,t,e,i){let s=!0;if(n.layers.test(t.layers)&&n.raycast(t,e)===!1&&(s=!1),s===!0&&i===!0){let r=n.children;for(let o=0,a=r.length;o<a;o++)sc(r[o],t,e,!0)}}var bo=class extends Ln{constructor(t,e=null){super(),this.object=t,this.domElement=e,this.enabled=!0,this.state=-1,this.keys={},this.mouseButtons={LEFT:null,MIDDLE:null,RIGHT:null},this.touches={ONE:null,TWO:null}}connect(){}disconnect(){}dispose(){}update(){}};typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("register",{detail:{revision:"169"}}));typeof window<"u"&&(window.__THREE__?console.warn("WARNING: Multiple instances of Three.js being imported."):window.__THREE__="169");var vc={type:"change"},Mc={type:"start"},Sc={type:"end"},Pu=1e-6,Yt={NONE:-1,ROTATE:0,ZOOM:1,PAN:2,TOUCH_ROTATE:3,TOUCH_ZOOM_PAN:4},To=new yt,Jn=new yt,Yx=new U,Co=new U,_c=new U,_s=new $e,Iu=new U,Ro=new U,yc=new U,Po=new U,Io=class extends bo{constructor(t,e=null){super(t,e),this.enabled=!0,this.screen={left:0,top:0,width:0,height:0},this.rotateSpeed=1,this.zoomSpeed=1.2,this.panSpeed=.3,this.noRotate=!1,this.noZoom=!1,this.noPan=!1,this.staticMoving=!1,this.dynamicDampingFactor=.2,this.minDistance=0,this.maxDistance=1/0,this.minZoom=0,this.maxZoom=1/0,this.keys=["KeyA","KeyS","KeyD"],this.mouseButtons={LEFT:bi.ROTATE,MIDDLE:bi.DOLLY,RIGHT:bi.PAN},this.state=Yt.NONE,this.keyState=Yt.NONE,this.target=new U,this._lastPosition=new U,this._lastZoom=1,this._touchZoomDistanceStart=0,this._touchZoomDistanceEnd=0,this._lastAngle=0,this._eye=new U,this._movePrev=new yt,this._moveCurr=new yt,this._lastAxis=new U,this._zoomStart=new yt,this._zoomEnd=new yt,this._panStart=new yt,this._panEnd=new yt,this._pointers=[],this._pointerPositions={},this._onPointerMove=jx.bind(this),this._onPointerDown=Zx.bind(this),this._onPointerUp=Kx.bind(this),this._onPointerCancel=Jx.bind(this),this._onContextMenu=sv.bind(this),this._onMouseWheel=iv.bind(this),this._onKeyDown=$x.bind(this),this._onKeyUp=Qx.bind(this),this._onTouchStart=rv.bind(this),this._onTouchMove=ov.bind(this),this._onTouchEnd=av.bind(this),this._onMouseDown=tv.bind(this),this._onMouseMove=ev.bind(this),this._onMouseUp=nv.bind(this),this._target0=this.target.clone(),this._position0=this.object.position.clone(),this._up0=this.object.up.clone(),this._zoom0=this.object.zoom,e!==null&&(this.connect(),this.handleResize()),this.update()}connect(){window.addEventListener("keydown",this._onKeyDown),window.addEventListener("keyup",this._onKeyUp),this.domElement.addEventListener("pointerdown",this._onPointerDown),this.domElement.addEventListener("pointercancel",this._onPointerCancel),this.domElement.addEventListener("wheel",this._onMouseWheel,{passive:!1}),this.domElement.addEventListener("contextmenu",this._onContextMenu),this.domElement.style.touchAction="none"}disconnect(){window.removeEventListener("keydown",this._onKeyDown),window.removeEventListener("keyup",this._onKeyUp),this.domElement.removeEventListener("pointerdown",this._onPointerDown),this.domElement.removeEventListener("pointermove",this._onPointerMove),this.domElement.removeEventListener("pointerup",this._onPointerUp),this.domElement.removeEventListener("pointercancel",this._onPointerCancel),this.domElement.removeEventListener("wheel",this._onMouseWheel),this.domElement.removeEventListener("contextmenu",this._onContextMenu),this.domElement.style.touchAction="auto"}dispose(){this.disconnect()}handleResize(){let t=this.domElement.getBoundingClientRect(),e=this.domElement.ownerDocument.documentElement;this.screen.left=t.left+window.pageXOffset-e.clientLeft,this.screen.top=t.top+window.pageYOffset-e.clientTop,this.screen.width=t.width,this.screen.height=t.height}update(){this._eye.subVectors(this.object.position,this.target),this.noRotate||this._rotateCamera(),this.noZoom||this._zoomCamera(),this.noPan||this._panCamera(),this.object.position.addVectors(this.target,this._eye),this.object.isPerspectiveCamera?(this._checkDistances(),this.object.lookAt(this.target),this._lastPosition.distanceToSquared(this.object.position)>Pu&&(this.dispatchEvent(vc),this._lastPosition.copy(this.object.position))):this.object.isOrthographicCamera?(this.object.lookAt(this.target),(this._lastPosition.distanceToSquared(this.object.position)>Pu||this._lastZoom!==this.object.zoom)&&(this.dispatchEvent(vc),this._lastPosition.copy(this.object.position),this._lastZoom=this.object.zoom)):console.warn("THREE.TrackballControls: Unsupported camera type.")}reset(){this.state=Yt.NONE,this.keyState=Yt.NONE,this.target.copy(this._target0),this.object.position.copy(this._position0),this.object.up.copy(this._up0),this.object.zoom=this._zoom0,this.object.updateProjectionMatrix(),this._eye.subVectors(this.object.position,this.target),this.object.lookAt(this.target),this.dispatchEvent(vc),this._lastPosition.copy(this.object.position),this._lastZoom=this.object.zoom}_panCamera(){if(Jn.copy(this._panEnd).sub(this._panStart),Jn.lengthSq()){if(this.object.isOrthographicCamera){let t=(this.object.right-this.object.left)/this.object.zoom/this.domElement.clientWidth,e=(this.object.top-this.object.bottom)/this.object.zoom/this.domElement.clientWidth;Jn.x*=t,Jn.y*=e}Jn.multiplyScalar(this._eye.length()*this.panSpeed),Co.copy(this._eye).cross(this.object.up).setLength(Jn.x),Co.add(Yx.copy(this.object.up).setLength(Jn.y)),this.object.position.add(Co),this.target.add(Co),this.staticMoving?this._panStart.copy(this._panEnd):this._panStart.add(Jn.subVectors(this._panEnd,this._panStart).multiplyScalar(this.dynamicDampingFactor))}}_rotateCamera(){Po.set(this._moveCurr.x-this._movePrev.x,this._moveCurr.y-this._movePrev.y,0);let t=Po.length();t?(this._eye.copy(this.object.position).sub(this.target),Iu.copy(this._eye).normalize(),Ro.copy(this.object.up).normalize(),yc.crossVectors(Ro,Iu).normalize(),Ro.setLength(this._moveCurr.y-this._movePrev.y),yc.setLength(this._moveCurr.x-this._movePrev.x),Po.copy(Ro.add(yc)),_c.crossVectors(Po,this._eye).normalize(),t*=this.rotateSpeed,_s.setFromAxisAngle(_c,t),this._eye.applyQuaternion(_s),this.object.up.applyQuaternion(_s),this._lastAxis.copy(_c),this._lastAngle=t):!this.staticMoving&&this._lastAngle&&(this._lastAngle*=Math.sqrt(1-this.dynamicDampingFactor),this._eye.copy(this.object.position).sub(this.target),_s.setFromAxisAngle(this._lastAxis,this._lastAngle),this._eye.applyQuaternion(_s),this.object.up.applyQuaternion(_s)),this._movePrev.copy(this._moveCurr)}_zoomCamera(){let t;this.state===Yt.TOUCH_ZOOM_PAN?(t=this._touchZoomDistanceStart/this._touchZoomDistanceEnd,this._touchZoomDistanceStart=this._touchZoomDistanceEnd,this.object.isPerspectiveCamera?this._eye.multiplyScalar(t):this.object.isOrthographicCamera?(this.object.zoom=pc.clamp(this.object.zoom/t,this.minZoom,this.maxZoom),this._lastZoom!==this.object.zoom&&this.object.updateProjectionMatrix()):console.warn("THREE.TrackballControls: Unsupported camera type")):(t=1+(this._zoomEnd.y-this._zoomStart.y)*this.zoomSpeed,t!==1&&t>0&&(this.object.isPerspectiveCamera?this._eye.multiplyScalar(t):this.object.isOrthographicCamera?(this.object.zoom=pc.clamp(this.object.zoom/t,this.minZoom,this.maxZoom),this._lastZoom!==this.object.zoom&&this.object.updateProjectionMatrix()):console.warn("THREE.TrackballControls: Unsupported camera type")),this.staticMoving?this._zoomStart.copy(this._zoomEnd):this._zoomStart.y+=(this._zoomEnd.y-this._zoomStart.y)*this.dynamicDampingFactor)}_getMouseOnScreen(t,e){return To.set((t-this.screen.left)/this.screen.width,(e-this.screen.top)/this.screen.height),To}_getMouseOnCircle(t,e){return To.set((t-this.screen.width*.5-this.screen.left)/(this.screen.width*.5),(this.screen.height+2*(this.screen.top-e))/this.screen.width),To}_addPointer(t){this._pointers.push(t)}_removePointer(t){delete this._pointerPositions[t.pointerId];for(let e=0;e<this._pointers.length;e++)if(this._pointers[e].pointerId==t.pointerId){this._pointers.splice(e,1);return}}_trackPointer(t){let e=this._pointerPositions[t.pointerId];e===void 0&&(e=new yt,this._pointerPositions[t.pointerId]=e),e.set(t.pageX,t.pageY)}_getSecondPointerPosition(t){let e=t.pointerId===this._pointers[0].pointerId?this._pointers[1]:this._pointers[0];return this._pointerPositions[e.pointerId]}_checkDistances(){(!this.noZoom||!this.noPan)&&(this._eye.lengthSq()>this.maxDistance*this.maxDistance&&(this.object.position.addVectors(this.target,this._eye.setLength(this.maxDistance)),this._zoomStart.copy(this._zoomEnd)),this._eye.lengthSq()<this.minDistance*this.minDistance&&(this.object.position.addVectors(this.target,this._eye.setLength(this.minDistance)),this._zoomStart.copy(this._zoomEnd)))}};function Zx(n){this.enabled!==!1&&(this._pointers.length===0&&(this.domElement.setPointerCapture(n.pointerId),this.domElement.addEventListener("pointermove",this._onPointerMove),this.domElement.addEventListener("pointerup",this._onPointerUp)),this._addPointer(n),n.pointerType==="touch"?this._onTouchStart(n):this._onMouseDown(n))}function jx(n){this.enabled!==!1&&(n.pointerType==="touch"?this._onTouchMove(n):this._onMouseMove(n))}function Kx(n){this.enabled!==!1&&(n.pointerType==="touch"?this._onTouchEnd(n):this._onMouseUp(),this._removePointer(n),this._pointers.length===0&&(this.domElement.releasePointerCapture(n.pointerId),this.domElement.removeEventListener("pointermove",this._onPointerMove),this.domElement.removeEventListener("pointerup",this._onPointerUp)))}function Jx(n){this._removePointer(n)}function Qx(){this.enabled!==!1&&(this.keyState=Yt.NONE,window.addEventListener("keydown",this._onKeyDown))}function $x(n){this.enabled!==!1&&(window.removeEventListener("keydown",this._onKeyDown),this.keyState===Yt.NONE&&(n.code===this.keys[Yt.ROTATE]&&!this.noRotate?this.keyState=Yt.ROTATE:n.code===this.keys[Yt.ZOOM]&&!this.noZoom?this.keyState=Yt.ZOOM:n.code===this.keys[Yt.PAN]&&!this.noPan&&(this.keyState=Yt.PAN)))}function tv(n){let t;switch(n.button){case 0:t=this.mouseButtons.LEFT;break;case 1:t=this.mouseButtons.MIDDLE;break;case 2:t=this.mouseButtons.RIGHT;break;default:t=-1}switch(t){case bi.DOLLY:this.state=Yt.ZOOM;break;case bi.ROTATE:this.state=Yt.ROTATE;break;case bi.PAN:this.state=Yt.PAN;break;default:this.state=Yt.NONE}let e=this.keyState!==Yt.NONE?this.keyState:this.state;e===Yt.ROTATE&&!this.noRotate?(this._moveCurr.copy(this._getMouseOnCircle(n.pageX,n.pageY)),this._movePrev.copy(this._moveCurr)):e===Yt.ZOOM&&!this.noZoom?(this._zoomStart.copy(this._getMouseOnScreen(n.pageX,n.pageY)),this._zoomEnd.copy(this._zoomStart)):e===Yt.PAN&&!this.noPan&&(this._panStart.copy(this._getMouseOnScreen(n.pageX,n.pageY)),this._panEnd.copy(this._panStart)),this.dispatchEvent(Mc)}function ev(n){let t=this.keyState!==Yt.NONE?this.keyState:this.state;t===Yt.ROTATE&&!this.noRotate?(this._movePrev.copy(this._moveCurr),this._moveCurr.copy(this._getMouseOnCircle(n.pageX,n.pageY))):t===Yt.ZOOM&&!this.noZoom?this._zoomEnd.copy(this._getMouseOnScreen(n.pageX,n.pageY)):t===Yt.PAN&&!this.noPan&&this._panEnd.copy(this._getMouseOnScreen(n.pageX,n.pageY))}function nv(){this.state=Yt.NONE,this.dispatchEvent(Sc)}function iv(n){if(this.enabled!==!1&&this.noZoom!==!0){switch(n.preventDefault(),n.deltaMode){case 2:this._zoomStart.y-=n.deltaY*.025;break;case 1:this._zoomStart.y-=n.deltaY*.01;break;default:this._zoomStart.y-=n.deltaY*25e-5;break}this.dispatchEvent(Mc),this.dispatchEvent(Sc)}}function sv(n){this.enabled!==!1&&n.preventDefault()}function rv(n){if(this._trackPointer(n),this._pointers.length===1)this.state=Yt.TOUCH_ROTATE,this._moveCurr.copy(this._getMouseOnCircle(this._pointers[0].pageX,this._pointers[0].pageY)),this._movePrev.copy(this._moveCurr);else{this.state=Yt.TOUCH_ZOOM_PAN;let t=this._pointers[0].pageX-this._pointers[1].pageX,e=this._pointers[0].pageY-this._pointers[1].pageY;this._touchZoomDistanceEnd=this._touchZoomDistanceStart=Math.sqrt(t*t+e*e);let i=(this._pointers[0].pageX+this._pointers[1].pageX)/2,s=(this._pointers[0].pageY+this._pointers[1].pageY)/2;this._panStart.copy(this._getMouseOnScreen(i,s)),this._panEnd.copy(this._panStart)}this.dispatchEvent(Mc)}function ov(n){if(this._trackPointer(n),this._pointers.length===1)this._movePrev.copy(this._moveCurr),this._moveCurr.copy(this._getMouseOnCircle(n.pageX,n.pageY));else{let t=this._getSecondPointerPosition(n),e=n.pageX-t.x,i=n.pageY-t.y;this._touchZoomDistanceEnd=Math.sqrt(e*e+i*i);let s=(n.pageX+t.x)/2,r=(n.pageY+t.y)/2;this._panEnd.copy(this._getMouseOnScreen(s,r))}}function av(n){switch(this._pointers.length){case 0:this.state=Yt.NONE;break;case 1:this.state=Yt.TOUCH_ROTATE,this._moveCurr.copy(this._getMouseOnCircle(n.pageX,n.pageY)),this._movePrev.copy(this._moveCurr);break;case 2:this.state=Yt.TOUCH_ZOOM_PAN;for(let t=0;t<this._pointers.length;t++)if(this._pointers[t].pointerId!==n.pointerId){let e=this._pointerPositions[this._pointers[t].pointerId];this._moveCurr.copy(this._getMouseOnCircle(e.x,e.y)),this._movePrev.copy(this._moveCurr);break}break}this.dispatchEvent(Sc)}var Lo={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

		varying vec2 vUv;

		void main() {

			vUv = uv;
			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

		}`,fragmentShader:`

		uniform float opacity;

		uniform sampler2D tDiffuse;

		varying vec2 vUv;

		void main() {

			vec4 texel = texture2D( tDiffuse, vUv );
			gl_FragColor = opacity * texel;


		}`};var Ye=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},lv=new ms(-1,1,1,-1,0,1),bc=class extends un{constructor(){super(),this.setAttribute("position",new ve([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new ve([0,2,0,0,2,0],2))}},cv=new bc,Qn=class{constructor(t){this._mesh=new Ue(cv,t)}dispose(){this._mesh.geometry.dispose()}render(t){t.render(this._mesh,lv)}get material(){return this._mesh.material}set material(t){this._mesh.material=t}};var Do=class extends Ye{constructor(t,e){super(),this.textureID=e!==void 0?e:"tDiffuse",t instanceof ie?(this.uniforms=t.uniforms,this.material=t):t&&(this.uniforms=_n.clone(t.uniforms),this.material=new ie({name:t.name!==void 0?t.name:"unspecified",defines:Object.assign({},t.defines),uniforms:this.uniforms,vertexShader:t.vertexShader,fragmentShader:t.fragmentShader})),this.fsQuad=new Qn(this.material)}render(t,e,i){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=i.texture),this.fsQuad.material=this.material,this.renderToScreen?(t.setRenderTarget(null),this.fsQuad.render(t)):(t.setRenderTarget(e),this.clear&&t.clear(t.autoClearColor,t.autoClearDepth,t.autoClearStencil),this.fsQuad.render(t))}dispose(){this.material.dispose(),this.fsQuad.dispose()}};var rr=class extends Ye{constructor(t,e){super(),this.scene=t,this.camera=e,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(t,e,i){let s=t.getContext(),r=t.state;r.buffers.color.setMask(!1),r.buffers.depth.setMask(!1),r.buffers.color.setLocked(!0),r.buffers.depth.setLocked(!0);let o,a;this.inverse?(o=0,a=1):(o=1,a=0),r.buffers.stencil.setTest(!0),r.buffers.stencil.setOp(s.REPLACE,s.REPLACE,s.REPLACE),r.buffers.stencil.setFunc(s.ALWAYS,o,4294967295),r.buffers.stencil.setClear(a),r.buffers.stencil.setLocked(!0),t.setRenderTarget(i),this.clear&&t.clear(),t.render(this.scene,this.camera),t.setRenderTarget(e),this.clear&&t.clear(),t.render(this.scene,this.camera),r.buffers.color.setLocked(!1),r.buffers.depth.setLocked(!1),r.buffers.color.setMask(!0),r.buffers.depth.setMask(!0),r.buffers.stencil.setLocked(!1),r.buffers.stencil.setFunc(s.EQUAL,1,4294967295),r.buffers.stencil.setOp(s.KEEP,s.KEEP,s.KEEP),r.buffers.stencil.setLocked(!0)}},Uo=class extends Ye{constructor(){super(),this.needsSwap=!1}render(t){t.state.buffers.stencil.setLocked(!1),t.state.buffers.stencil.setTest(!1)}};var No=class{constructor(t,e){if(this.renderer=t,this._pixelRatio=t.getPixelRatio(),e===void 0){let i=t.getSize(new yt);this._width=i.width,this._height=i.height,e=new ue(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:ze}),e.texture.name="EffectComposer.rt1"}else this._width=e.width,this._height=e.height;this.renderTarget1=e,this.renderTarget2=e.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new Do(Lo),this.copyPass.material.blending=vn,this.clock=new Mo}swapBuffers(){let t=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=t}addPass(t){this.passes.push(t),t.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(t,e){this.passes.splice(e,0,t),t.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(t){let e=this.passes.indexOf(t);e!==-1&&this.passes.splice(e,1)}isLastEnabledPass(t){for(let e=t+1;e<this.passes.length;e++)if(this.passes[e].enabled)return!1;return!0}render(t){t===void 0&&(t=this.clock.getDelta());let e=this.renderer.getRenderTarget(),i=!1;for(let s=0,r=this.passes.length;s<r;s++){let o=this.passes[s];if(o.enabled!==!1){if(o.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(s),o.render(this.renderer,this.writeBuffer,this.readBuffer,t,i),o.needsSwap){if(i){let a=this.renderer.getContext(),l=this.renderer.state.buffers.stencil;l.setFunc(a.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,t),l.setFunc(a.EQUAL,1,4294967295)}this.swapBuffers()}rr!==void 0&&(o instanceof rr?i=!0:o instanceof Uo&&(i=!1))}}this.renderer.setRenderTarget(e)}reset(t){if(t===void 0){let e=this.renderer.getSize(new yt);this._pixelRatio=this.renderer.getPixelRatio(),this._width=e.width,this._height=e.height,t=this.renderTarget1.clone(),t.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=t,this.renderTarget2=t.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(t,e){this._width=t,this._height=e;let i=this._width*this._pixelRatio,s=this._height*this._pixelRatio;this.renderTarget1.setSize(i,s),this.renderTarget2.setSize(i,s);for(let r=0;r<this.passes.length;r++)this.passes[r].setSize(i,s)}setPixelRatio(t){this._pixelRatio=t,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};var Oo=class extends Ye{constructor(t,e,i=null,s=null,r=null){super(),this.scene=t,this.camera=e,this.overrideMaterial=i,this.clearColor=s,this.clearAlpha=r,this.clear=!0,this.clearDepth=!1,this.needsSwap=!1,this._oldClearColor=new Ot}render(t,e,i){let s=t.autoClear;t.autoClear=!1;let r,o;this.overrideMaterial!==null&&(o=this.scene.overrideMaterial,this.scene.overrideMaterial=this.overrideMaterial),this.clearColor!==null&&(t.getClearColor(this._oldClearColor),t.setClearColor(this.clearColor,t.getClearAlpha())),this.clearAlpha!==null&&(r=t.getClearAlpha(),t.setClearAlpha(this.clearAlpha)),this.clearDepth==!0&&t.clearDepth(),t.setRenderTarget(this.renderToScreen?null:i),this.clear===!0&&t.clear(t.autoClearColor,t.autoClearDepth,t.autoClearStencil),t.render(this.scene,this.camera),this.clearColor!==null&&t.setClearColor(this._oldClearColor),this.clearAlpha!==null&&t.setClearAlpha(r),this.overrideMaterial!==null&&(this.scene.overrideMaterial=o),t.autoClear=s}};var Lu={name:"LuminosityHighPassShader",shaderID:"luminosityHighPass",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new Ot(0)},defaultOpacity:{value:0}},vertexShader:`

		varying vec2 vUv;

		void main() {

			vUv = uv;

			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

		}`,fragmentShader:`

		uniform sampler2D tDiffuse;
		uniform vec3 defaultColor;
		uniform float defaultOpacity;
		uniform float luminosityThreshold;
		uniform float smoothWidth;

		varying vec2 vUv;

		void main() {

			vec4 texel = texture2D( tDiffuse, vUv );

			float v = luminance( texel.xyz );

			vec4 outputColor = vec4( defaultColor.rgb, defaultOpacity );

			float alpha = smoothstep( luminosityThreshold, luminosityThreshold + smoothWidth, v );

			gl_FragColor = mix( outputColor, texel, alpha );

		}`};var ys=class n extends Ye{constructor(t,e,i,s){super(),this.strength=e!==void 0?e:1,this.radius=i,this.threshold=s,this.resolution=t!==void 0?new yt(t.x,t.y):new yt(256,256),this.clearColor=new Ot(0,0,0),this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);this.renderTargetBright=new ue(r,o,{type:ze}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let d=0;d<this.nMips;d++){let c=new ue(r,o,{type:ze});c.texture.name="UnrealBloomPass.h"+d,c.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(c);let u=new ue(r,o,{type:ze});u.texture.name="UnrealBloomPass.v"+d,u.texture.generateMipmaps=!1,this.renderTargetsVertical.push(u),r=Math.round(r/2),o=Math.round(o/2)}let a=Lu;this.highPassUniforms=_n.clone(a.uniforms),this.highPassUniforms.luminosityThreshold.value=s,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new ie({uniforms:this.highPassUniforms,vertexShader:a.vertexShader,fragmentShader:a.fragmentShader}),this.separableBlurMaterials=[];let l=[3,5,7,9,11];r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);for(let d=0;d<this.nMips;d++)this.separableBlurMaterials.push(this.getSeperableBlurMaterial(l[d])),this.separableBlurMaterials[d].uniforms.invSize.value=new yt(1/r,1/o),r=Math.round(r/2),o=Math.round(o/2);this.compositeMaterial=this.getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=e,this.compositeMaterial.uniforms.bloomRadius.value=.1;let h=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=h,this.bloomTintColors=[new U(1,1,1),new U(1,1,1),new U(1,1,1),new U(1,1,1),new U(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors;let f=Lo;this.copyUniforms=_n.clone(f.uniforms),this.blendMaterial=new ie({uniforms:this.copyUniforms,vertexShader:f.vertexShader,fragmentShader:f.fragmentShader,blending:as,depthTest:!1,depthWrite:!1,transparent:!0}),this.enabled=!0,this.needsSwap=!1,this._oldClearColor=new Ot,this.oldClearAlpha=1,this.basic=new fs,this.fsQuad=new Qn(null)}dispose(){for(let t=0;t<this.renderTargetsHorizontal.length;t++)this.renderTargetsHorizontal[t].dispose();for(let t=0;t<this.renderTargetsVertical.length;t++)this.renderTargetsVertical[t].dispose();this.renderTargetBright.dispose();for(let t=0;t<this.separableBlurMaterials.length;t++)this.separableBlurMaterials[t].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this.basic.dispose(),this.fsQuad.dispose()}setSize(t,e){let i=Math.round(t/2),s=Math.round(e/2);this.renderTargetBright.setSize(i,s);for(let r=0;r<this.nMips;r++)this.renderTargetsHorizontal[r].setSize(i,s),this.renderTargetsVertical[r].setSize(i,s),this.separableBlurMaterials[r].uniforms.invSize.value=new yt(1/i,1/s),i=Math.round(i/2),s=Math.round(s/2)}render(t,e,i,s,r){t.getClearColor(this._oldClearColor),this.oldClearAlpha=t.getClearAlpha();let o=t.autoClear;t.autoClear=!1,t.setClearColor(this.clearColor,0),r&&t.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this.fsQuad.material=this.basic,this.basic.map=i.texture,t.setRenderTarget(null),t.clear(),this.fsQuad.render(t)),this.highPassUniforms.tDiffuse.value=i.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this.fsQuad.material=this.materialHighPassFilter,t.setRenderTarget(this.renderTargetBright),t.clear(),this.fsQuad.render(t);let a=this.renderTargetBright;for(let l=0;l<this.nMips;l++)this.fsQuad.material=this.separableBlurMaterials[l],this.separableBlurMaterials[l].uniforms.colorTexture.value=a.texture,this.separableBlurMaterials[l].uniforms.direction.value=n.BlurDirectionX,t.setRenderTarget(this.renderTargetsHorizontal[l]),t.clear(),this.fsQuad.render(t),this.separableBlurMaterials[l].uniforms.colorTexture.value=this.renderTargetsHorizontal[l].texture,this.separableBlurMaterials[l].uniforms.direction.value=n.BlurDirectionY,t.setRenderTarget(this.renderTargetsVertical[l]),t.clear(),this.fsQuad.render(t),a=this.renderTargetsVertical[l];this.fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,t.setRenderTarget(this.renderTargetsHorizontal[0]),t.clear(),this.fsQuad.render(t),this.fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,r&&t.state.buffers.stencil.setTest(!0),this.renderToScreen?(t.setRenderTarget(null),this.fsQuad.render(t)):(t.setRenderTarget(i),this.fsQuad.render(t)),t.setClearColor(this._oldClearColor,this.oldClearAlpha),t.autoClear=o}getSeperableBlurMaterial(t){let e=[];for(let i=0;i<t;i++)e.push(.39894*Math.exp(-.5*i*i/(t*t))/t);return new ie({defines:{KERNEL_RADIUS:t},uniforms:{colorTexture:{value:null},invSize:{value:new yt(.5,.5)},direction:{value:new yt(.5,.5)},gaussianCoefficients:{value:e}},vertexShader:`varying vec2 vUv;
				void main() {
					vUv = uv;
					gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
				}`,fragmentShader:`#include <common>
				varying vec2 vUv;
				uniform sampler2D colorTexture;
				uniform vec2 invSize;
				uniform vec2 direction;
				uniform float gaussianCoefficients[KERNEL_RADIUS];

				void main() {
					float weightSum = gaussianCoefficients[0];
					vec3 diffuseSum = texture2D( colorTexture, vUv ).rgb * weightSum;
					for( int i = 1; i < KERNEL_RADIUS; i ++ ) {
						float x = float(i);
						float w = gaussianCoefficients[i];
						vec2 uvOffset = direction * invSize * x;
						vec3 sample1 = texture2D( colorTexture, vUv + uvOffset ).rgb;
						vec3 sample2 = texture2D( colorTexture, vUv - uvOffset ).rgb;
						diffuseSum += (sample1 + sample2) * w;
						weightSum += 2.0 * w;
					}
					gl_FragColor = vec4(diffuseSum/weightSum, 1.0);
				}`})}getCompositeMaterial(t){return new ie({defines:{NUM_MIPS:t},uniforms:{blurTexture1:{value:null},blurTexture2:{value:null},blurTexture3:{value:null},blurTexture4:{value:null},blurTexture5:{value:null},bloomStrength:{value:1},bloomFactors:{value:null},bloomTintColors:{value:null},bloomRadius:{value:0}},vertexShader:`varying vec2 vUv;
				void main() {
					vUv = uv;
					gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
				}`,fragmentShader:`varying vec2 vUv;
				uniform sampler2D blurTexture1;
				uniform sampler2D blurTexture2;
				uniform sampler2D blurTexture3;
				uniform sampler2D blurTexture4;
				uniform sampler2D blurTexture5;
				uniform float bloomStrength;
				uniform float bloomRadius;
				uniform float bloomFactors[NUM_MIPS];
				uniform vec3 bloomTintColors[NUM_MIPS];

				float lerpBloomFactor(const in float factor) {
					float mirrorFactor = 1.2 - factor;
					return mix(factor, mirrorFactor, bloomRadius);
				}

				void main() {
					gl_FragColor = bloomStrength * ( lerpBloomFactor(bloomFactors[0]) * vec4(bloomTintColors[0], 1.0) * texture2D(blurTexture1, vUv) +
						lerpBloomFactor(bloomFactors[1]) * vec4(bloomTintColors[1], 1.0) * texture2D(blurTexture2, vUv) +
						lerpBloomFactor(bloomFactors[2]) * vec4(bloomTintColors[2], 1.0) * texture2D(blurTexture3, vUv) +
						lerpBloomFactor(bloomFactors[3]) * vec4(bloomTintColors[3], 1.0) * texture2D(blurTexture4, vUv) +
						lerpBloomFactor(bloomFactors[4]) * vec4(bloomTintColors[4], 1.0) * texture2D(blurTexture5, vUv) );
				}`})}};ys.BlurDirectionX=new yt(1,0);ys.BlurDirectionY=new yt(0,1);var or={name:"SMAAEdgesShader",defines:{SMAA_THRESHOLD:"0.1"},uniforms:{tDiffuse:{value:null},resolution:{value:new yt(1/1024,1/512)}},vertexShader:`

		uniform vec2 resolution;

		varying vec2 vUv;
		varying vec4 vOffset[ 3 ];

		void SMAAEdgeDetectionVS( vec2 texcoord ) {
			vOffset[ 0 ] = texcoord.xyxy + resolution.xyxy * vec4( -1.0, 0.0, 0.0,  1.0 ); // WebGL port note: Changed sign in W component
			vOffset[ 1 ] = texcoord.xyxy + resolution.xyxy * vec4(  1.0, 0.0, 0.0, -1.0 ); // WebGL port note: Changed sign in W component
			vOffset[ 2 ] = texcoord.xyxy + resolution.xyxy * vec4( -2.0, 0.0, 0.0,  2.0 ); // WebGL port note: Changed sign in W component
		}

		void main() {

			vUv = uv;

			SMAAEdgeDetectionVS( vUv );

			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

		}`,fragmentShader:`

		uniform sampler2D tDiffuse;

		varying vec2 vUv;
		varying vec4 vOffset[ 3 ];

		vec4 SMAAColorEdgeDetectionPS( vec2 texcoord, vec4 offset[3], sampler2D colorTex ) {
			vec2 threshold = vec2( SMAA_THRESHOLD, SMAA_THRESHOLD );

			// Calculate color deltas:
			vec4 delta;
			vec3 C = texture2D( colorTex, texcoord ).rgb;

			vec3 Cleft = texture2D( colorTex, offset[0].xy ).rgb;
			vec3 t = abs( C - Cleft );
			delta.x = max( max( t.r, t.g ), t.b );

			vec3 Ctop = texture2D( colorTex, offset[0].zw ).rgb;
			t = abs( C - Ctop );
			delta.y = max( max( t.r, t.g ), t.b );

			// We do the usual threshold:
			vec2 edges = step( threshold, delta.xy );

			// Then discard if there is no edge:
			if ( dot( edges, vec2( 1.0, 1.0 ) ) == 0.0 )
				discard;

			// Calculate right and bottom deltas:
			vec3 Cright = texture2D( colorTex, offset[1].xy ).rgb;
			t = abs( C - Cright );
			delta.z = max( max( t.r, t.g ), t.b );

			vec3 Cbottom  = texture2D( colorTex, offset[1].zw ).rgb;
			t = abs( C - Cbottom );
			delta.w = max( max( t.r, t.g ), t.b );

			// Calculate the maximum delta in the direct neighborhood:
			float maxDelta = max( max( max( delta.x, delta.y ), delta.z ), delta.w );

			// Calculate left-left and top-top deltas:
			vec3 Cleftleft  = texture2D( colorTex, offset[2].xy ).rgb;
			t = abs( C - Cleftleft );
			delta.z = max( max( t.r, t.g ), t.b );

			vec3 Ctoptop = texture2D( colorTex, offset[2].zw ).rgb;
			t = abs( C - Ctoptop );
			delta.w = max( max( t.r, t.g ), t.b );

			// Calculate the final maximum delta:
			maxDelta = max( max( maxDelta, delta.z ), delta.w );

			// Local contrast adaptation in action:
			edges.xy *= step( 0.5 * maxDelta, delta.xy );

			return vec4( edges, 0.0, 0.0 );
		}

		void main() {

			gl_FragColor = SMAAColorEdgeDetectionPS( vUv, vOffset, tDiffuse );

		}`},ar={name:"SMAAWeightsShader",defines:{SMAA_MAX_SEARCH_STEPS:"8",SMAA_AREATEX_MAX_DISTANCE:"16",SMAA_AREATEX_PIXEL_SIZE:"( 1.0 / vec2( 160.0, 560.0 ) )",SMAA_AREATEX_SUBTEX_SIZE:"( 1.0 / 7.0 )"},uniforms:{tDiffuse:{value:null},tArea:{value:null},tSearch:{value:null},resolution:{value:new yt(1/1024,1/512)}},vertexShader:`

		uniform vec2 resolution;

		varying vec2 vUv;
		varying vec4 vOffset[ 3 ];
		varying vec2 vPixcoord;

		void SMAABlendingWeightCalculationVS( vec2 texcoord ) {
			vPixcoord = texcoord / resolution;

			// We will use these offsets for the searches later on (see @PSEUDO_GATHER4):
			vOffset[ 0 ] = texcoord.xyxy + resolution.xyxy * vec4( -0.25, 0.125, 1.25, 0.125 ); // WebGL port note: Changed sign in Y and W components
			vOffset[ 1 ] = texcoord.xyxy + resolution.xyxy * vec4( -0.125, 0.25, -0.125, -1.25 ); // WebGL port note: Changed sign in Y and W components

			// And these for the searches, they indicate the ends of the loops:
			vOffset[ 2 ] = vec4( vOffset[ 0 ].xz, vOffset[ 1 ].yw ) + vec4( -2.0, 2.0, -2.0, 2.0 ) * resolution.xxyy * float( SMAA_MAX_SEARCH_STEPS );

		}

		void main() {

			vUv = uv;

			SMAABlendingWeightCalculationVS( vUv );

			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

		}`,fragmentShader:`

		#define SMAASampleLevelZeroOffset( tex, coord, offset ) texture2D( tex, coord + float( offset ) * resolution, 0.0 )

		uniform sampler2D tDiffuse;
		uniform sampler2D tArea;
		uniform sampler2D tSearch;
		uniform vec2 resolution;

		varying vec2 vUv;
		varying vec4 vOffset[3];
		varying vec2 vPixcoord;

		#if __VERSION__ == 100
		vec2 round( vec2 x ) {
			return sign( x ) * floor( abs( x ) + 0.5 );
		}
		#endif

		float SMAASearchLength( sampler2D searchTex, vec2 e, float bias, float scale ) {
			// Not required if searchTex accesses are set to point:
			// float2 SEARCH_TEX_PIXEL_SIZE = 1.0 / float2(66.0, 33.0);
			// e = float2(bias, 0.0) + 0.5 * SEARCH_TEX_PIXEL_SIZE +
			//     e * float2(scale, 1.0) * float2(64.0, 32.0) * SEARCH_TEX_PIXEL_SIZE;
			e.r = bias + e.r * scale;
			return 255.0 * texture2D( searchTex, e, 0.0 ).r;
		}

		float SMAASearchXLeft( sampler2D edgesTex, sampler2D searchTex, vec2 texcoord, float end ) {
			/**
				* @PSEUDO_GATHER4
				* This texcoord has been offset by (-0.25, -0.125) in the vertex shader to
				* sample between edge, thus fetching four edges in a row.
				* Sampling with different offsets in each direction allows to disambiguate
				* which edges are active from the four fetched ones.
				*/
			vec2 e = vec2( 0.0, 1.0 );

			for ( int i = 0; i < SMAA_MAX_SEARCH_STEPS; i ++ ) { // WebGL port note: Changed while to for
				e = texture2D( edgesTex, texcoord, 0.0 ).rg;
				texcoord -= vec2( 2.0, 0.0 ) * resolution;
				if ( ! ( texcoord.x > end && e.g > 0.8281 && e.r == 0.0 ) ) break;
			}

			// We correct the previous (-0.25, -0.125) offset we applied:
			texcoord.x += 0.25 * resolution.x;

			// The searches are bias by 1, so adjust the coords accordingly:
			texcoord.x += resolution.x;

			// Disambiguate the length added by the last step:
			texcoord.x += 2.0 * resolution.x; // Undo last step
			texcoord.x -= resolution.x * SMAASearchLength(searchTex, e, 0.0, 0.5);

			return texcoord.x;
		}

		float SMAASearchXRight( sampler2D edgesTex, sampler2D searchTex, vec2 texcoord, float end ) {
			vec2 e = vec2( 0.0, 1.0 );

			for ( int i = 0; i < SMAA_MAX_SEARCH_STEPS; i ++ ) { // WebGL port note: Changed while to for
				e = texture2D( edgesTex, texcoord, 0.0 ).rg;
				texcoord += vec2( 2.0, 0.0 ) * resolution;
				if ( ! ( texcoord.x < end && e.g > 0.8281 && e.r == 0.0 ) ) break;
			}

			texcoord.x -= 0.25 * resolution.x;
			texcoord.x -= resolution.x;
			texcoord.x -= 2.0 * resolution.x;
			texcoord.x += resolution.x * SMAASearchLength( searchTex, e, 0.5, 0.5 );

			return texcoord.x;
		}

		float SMAASearchYUp( sampler2D edgesTex, sampler2D searchTex, vec2 texcoord, float end ) {
			vec2 e = vec2( 1.0, 0.0 );

			for ( int i = 0; i < SMAA_MAX_SEARCH_STEPS; i ++ ) { // WebGL port note: Changed while to for
				e = texture2D( edgesTex, texcoord, 0.0 ).rg;
				texcoord += vec2( 0.0, 2.0 ) * resolution; // WebGL port note: Changed sign
				if ( ! ( texcoord.y > end && e.r > 0.8281 && e.g == 0.0 ) ) break;
			}

			texcoord.y -= 0.25 * resolution.y; // WebGL port note: Changed sign
			texcoord.y -= resolution.y; // WebGL port note: Changed sign
			texcoord.y -= 2.0 * resolution.y; // WebGL port note: Changed sign
			texcoord.y += resolution.y * SMAASearchLength( searchTex, e.gr, 0.0, 0.5 ); // WebGL port note: Changed sign

			return texcoord.y;
		}

		float SMAASearchYDown( sampler2D edgesTex, sampler2D searchTex, vec2 texcoord, float end ) {
			vec2 e = vec2( 1.0, 0.0 );

			for ( int i = 0; i < SMAA_MAX_SEARCH_STEPS; i ++ ) { // WebGL port note: Changed while to for
				e = texture2D( edgesTex, texcoord, 0.0 ).rg;
				texcoord -= vec2( 0.0, 2.0 ) * resolution; // WebGL port note: Changed sign
				if ( ! ( texcoord.y < end && e.r > 0.8281 && e.g == 0.0 ) ) break;
			}

			texcoord.y += 0.25 * resolution.y; // WebGL port note: Changed sign
			texcoord.y += resolution.y; // WebGL port note: Changed sign
			texcoord.y += 2.0 * resolution.y; // WebGL port note: Changed sign
			texcoord.y -= resolution.y * SMAASearchLength( searchTex, e.gr, 0.5, 0.5 ); // WebGL port note: Changed sign

			return texcoord.y;
		}

		vec2 SMAAArea( sampler2D areaTex, vec2 dist, float e1, float e2, float offset ) {
			// Rounding prevents precision errors of bilinear filtering:
			vec2 texcoord = float( SMAA_AREATEX_MAX_DISTANCE ) * round( 4.0 * vec2( e1, e2 ) ) + dist;

			// We do a scale and bias for mapping to texel space:
			texcoord = SMAA_AREATEX_PIXEL_SIZE * texcoord + ( 0.5 * SMAA_AREATEX_PIXEL_SIZE );

			// Move to proper place, according to the subpixel offset:
			texcoord.y += SMAA_AREATEX_SUBTEX_SIZE * offset;

			return texture2D( areaTex, texcoord, 0.0 ).rg;
		}

		vec4 SMAABlendingWeightCalculationPS( vec2 texcoord, vec2 pixcoord, vec4 offset[ 3 ], sampler2D edgesTex, sampler2D areaTex, sampler2D searchTex, ivec4 subsampleIndices ) {
			vec4 weights = vec4( 0.0, 0.0, 0.0, 0.0 );

			vec2 e = texture2D( edgesTex, texcoord ).rg;

			if ( e.g > 0.0 ) { // Edge at north
				vec2 d;

				// Find the distance to the left:
				vec2 coords;
				coords.x = SMAASearchXLeft( edgesTex, searchTex, offset[ 0 ].xy, offset[ 2 ].x );
				coords.y = offset[ 1 ].y; // offset[1].y = texcoord.y - 0.25 * resolution.y (@CROSSING_OFFSET)
				d.x = coords.x;

				// Now fetch the left crossing edges, two at a time using bilinear
				// filtering. Sampling at -0.25 (see @CROSSING_OFFSET) enables to
				// discern what value each edge has:
				float e1 = texture2D( edgesTex, coords, 0.0 ).r;

				// Find the distance to the right:
				coords.x = SMAASearchXRight( edgesTex, searchTex, offset[ 0 ].zw, offset[ 2 ].y );
				d.y = coords.x;

				// We want the distances to be in pixel units (doing this here allow to
				// better interleave arithmetic and memory accesses):
				d = d / resolution.x - pixcoord.x;

				// SMAAArea below needs a sqrt, as the areas texture is compressed
				// quadratically:
				vec2 sqrt_d = sqrt( abs( d ) );

				// Fetch the right crossing edges:
				coords.y -= 1.0 * resolution.y; // WebGL port note: Added
				float e2 = SMAASampleLevelZeroOffset( edgesTex, coords, ivec2( 1, 0 ) ).r;

				// Ok, we know how this pattern looks like, now it is time for getting
				// the actual area:
				weights.rg = SMAAArea( areaTex, sqrt_d, e1, e2, float( subsampleIndices.y ) );
			}

			if ( e.r > 0.0 ) { // Edge at west
				vec2 d;

				// Find the distance to the top:
				vec2 coords;

				coords.y = SMAASearchYUp( edgesTex, searchTex, offset[ 1 ].xy, offset[ 2 ].z );
				coords.x = offset[ 0 ].x; // offset[1].x = texcoord.x - 0.25 * resolution.x;
				d.x = coords.y;

				// Fetch the top crossing edges:
				float e1 = texture2D( edgesTex, coords, 0.0 ).g;

				// Find the distance to the bottom:
				coords.y = SMAASearchYDown( edgesTex, searchTex, offset[ 1 ].zw, offset[ 2 ].w );
				d.y = coords.y;

				// We want the distances to be in pixel units:
				d = d / resolution.y - pixcoord.y;

				// SMAAArea below needs a sqrt, as the areas texture is compressed
				// quadratically:
				vec2 sqrt_d = sqrt( abs( d ) );

				// Fetch the bottom crossing edges:
				coords.y -= 1.0 * resolution.y; // WebGL port note: Added
				float e2 = SMAASampleLevelZeroOffset( edgesTex, coords, ivec2( 0, 1 ) ).g;

				// Get the area for this direction:
				weights.ba = SMAAArea( areaTex, sqrt_d, e1, e2, float( subsampleIndices.x ) );
			}

			return weights;
		}

		void main() {

			gl_FragColor = SMAABlendingWeightCalculationPS( vUv, vPixcoord, vOffset, tDiffuse, tArea, tSearch, ivec4( 0.0 ) );

		}`},Fo={name:"SMAABlendShader",uniforms:{tDiffuse:{value:null},tColor:{value:null},resolution:{value:new yt(1/1024,1/512)}},vertexShader:`

		uniform vec2 resolution;

		varying vec2 vUv;
		varying vec4 vOffset[ 2 ];

		void SMAANeighborhoodBlendingVS( vec2 texcoord ) {
			vOffset[ 0 ] = texcoord.xyxy + resolution.xyxy * vec4( -1.0, 0.0, 0.0, 1.0 ); // WebGL port note: Changed sign in W component
			vOffset[ 1 ] = texcoord.xyxy + resolution.xyxy * vec4( 1.0, 0.0, 0.0, -1.0 ); // WebGL port note: Changed sign in W component
		}

		void main() {

			vUv = uv;

			SMAANeighborhoodBlendingVS( vUv );

			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );

		}`,fragmentShader:`

		uniform sampler2D tDiffuse;
		uniform sampler2D tColor;
		uniform vec2 resolution;

		varying vec2 vUv;
		varying vec4 vOffset[ 2 ];

		vec4 SMAANeighborhoodBlendingPS( vec2 texcoord, vec4 offset[ 2 ], sampler2D colorTex, sampler2D blendTex ) {
			// Fetch the blending weights for current pixel:
			vec4 a;
			a.xz = texture2D( blendTex, texcoord ).xz;
			a.y = texture2D( blendTex, offset[ 1 ].zw ).g;
			a.w = texture2D( blendTex, offset[ 1 ].xy ).a;

			// Is there any blending weight with a value greater than 0.0?
			if ( dot(a, vec4( 1.0, 1.0, 1.0, 1.0 )) < 1e-5 ) {
				return texture2D( colorTex, texcoord, 0.0 );
			} else {
				// Up to 4 lines can be crossing a pixel (one through each edge). We
				// favor blending by choosing the line with the maximum weight for each
				// direction:
				vec2 offset;
				offset.x = a.a > a.b ? a.a : -a.b; // left vs. right
				offset.y = a.g > a.r ? -a.g : a.r; // top vs. bottom // WebGL port note: Changed signs

				// Then we go in the direction that has the maximum weight:
				if ( abs( offset.x ) > abs( offset.y )) { // horizontal vs. vertical
					offset.y = 0.0;
				} else {
					offset.x = 0.0;
				}

				// Fetch the opposite color and lerp by hand:
				vec4 C = texture2D( colorTex, texcoord, 0.0 );
				texcoord += sign( offset ) * resolution;
				vec4 Cop = texture2D( colorTex, texcoord, 0.0 );
				float s = abs( offset.x ) > abs( offset.y ) ? abs( offset.x ) : abs( offset.y );

				// WebGL port note: Added gamma correction
				C.xyz = pow(C.xyz, vec3(2.2));
				Cop.xyz = pow(Cop.xyz, vec3(2.2));
				vec4 mixed = mix(C, Cop, s);
				mixed.xyz = pow(mixed.xyz, vec3(1.0 / 2.2));

				return mixed;
			}
		}

		void main() {

			gl_FragColor = SMAANeighborhoodBlendingPS( vUv, vOffset, tColor, tDiffuse );

		}`};var Bo=class extends Ye{constructor(t,e){super(),this.edgesRT=new ue(t,e,{depthBuffer:!1,type:ze}),this.edgesRT.texture.name="SMAAPass.edges",this.weightsRT=new ue(t,e,{depthBuffer:!1,type:ze}),this.weightsRT.texture.name="SMAAPass.weights";let i=this,s=new Image;s.src=this.getAreaTexture(),s.onload=function(){i.areaTexture.needsUpdate=!0},this.areaTexture=new Ce,this.areaTexture.name="SMAAPass.area",this.areaTexture.image=s,this.areaTexture.minFilter=Xe,this.areaTexture.generateMipmaps=!1,this.areaTexture.flipY=!1;let r=new Image;r.src=this.getSearchTexture(),r.onload=function(){i.searchTexture.needsUpdate=!0},this.searchTexture=new Ce,this.searchTexture.name="SMAAPass.search",this.searchTexture.image=r,this.searchTexture.magFilter=be,this.searchTexture.minFilter=be,this.searchTexture.generateMipmaps=!1,this.searchTexture.flipY=!1,this.uniformsEdges=_n.clone(or.uniforms),this.uniformsEdges.resolution.value.set(1/t,1/e),this.materialEdges=new ie({defines:Object.assign({},or.defines),uniforms:this.uniformsEdges,vertexShader:or.vertexShader,fragmentShader:or.fragmentShader}),this.uniformsWeights=_n.clone(ar.uniforms),this.uniformsWeights.resolution.value.set(1/t,1/e),this.uniformsWeights.tDiffuse.value=this.edgesRT.texture,this.uniformsWeights.tArea.value=this.areaTexture,this.uniformsWeights.tSearch.value=this.searchTexture,this.materialWeights=new ie({defines:Object.assign({},ar.defines),uniforms:this.uniformsWeights,vertexShader:ar.vertexShader,fragmentShader:ar.fragmentShader}),this.uniformsBlend=_n.clone(Fo.uniforms),this.uniformsBlend.resolution.value.set(1/t,1/e),this.uniformsBlend.tDiffuse.value=this.weightsRT.texture,this.materialBlend=new ie({uniforms:this.uniformsBlend,vertexShader:Fo.vertexShader,fragmentShader:Fo.fragmentShader}),this.fsQuad=new Qn(null)}render(t,e,i){this.uniformsEdges.tDiffuse.value=i.texture,this.fsQuad.material=this.materialEdges,t.setRenderTarget(this.edgesRT),this.clear&&t.clear(),this.fsQuad.render(t),this.fsQuad.material=this.materialWeights,t.setRenderTarget(this.weightsRT),this.clear&&t.clear(),this.fsQuad.render(t),this.uniformsBlend.tColor.value=i.texture,this.fsQuad.material=this.materialBlend,this.renderToScreen?(t.setRenderTarget(null),this.fsQuad.render(t)):(t.setRenderTarget(e),this.clear&&t.clear(),this.fsQuad.render(t))}setSize(t,e){this.edgesRT.setSize(t,e),this.weightsRT.setSize(t,e),this.materialEdges.uniforms.resolution.value.set(1/t,1/e),this.materialWeights.uniforms.resolution.value.set(1/t,1/e),this.materialBlend.uniforms.resolution.value.set(1/t,1/e)}getAreaTexture(){return"data:image/png;base64,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"}getSearchTexture(){return"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEIAAAAhCAAAAABIXyLAAAAAOElEQVRIx2NgGAWjYBSMglEwEICREYRgFBZBqDCSLA2MGPUIVQETE9iNUAqLR5gIeoQKRgwXjwAAGn4AtaFeYLEAAAAASUVORK5CYII="}dispose(){this.edgesRT.dispose(),this.weightsRT.dispose(),this.areaTexture.dispose(),this.searchTexture.dispose(),this.materialEdges.dispose(),this.materialWeights.dispose(),this.materialBlend.dispose(),this.fsQuad.dispose()}};var Ou=["PLANIFICADOR","REFUTADOR","SALA DE MANDO","PRINCIPAL","EMISARIO","AUDITOR","SKILLS","CUERPO","ALTURA","DAVANTIS","GIO"],hv=[...Ou].sort((n,t)=>t.length-n.length);function Ms(n){if(!n)return"";let t=n.indexOf("-");return(t===-1?n:n.slice(0,t)).toLowerCase()}var ko={"davantis-mando-admin":"SALA DE MANDO","davantis-mando":"SALA DE MANDO","davantis-altura":"ALTURA",davantis:"DAVANTIS",gio:"GIO"},zo="(servicios)";function Fu(n,t){let e=t&&t.actores||[],i=new Map((n||[]).map(o=>[o.id,o])),s=[],r=0;for(let o of e){let a=String(o&&o.principal||"").toLowerCase();if(!a)continue;let l={principal:a,calls:Math.max(0,Number(o.calls)||0),sondeo:Math.max(0,Number(o.sondeo)||0),trabajo:Math.max(0,Number(o.trabajo)||0),errores:Math.max(0,Number(o.errors)||0)+Math.max(0,Number(o.denied)||0),tools:Math.max(0,Number(o.tools)||0),proyecto:String(o.project||"")},h=ko[a],f=h&&i.get(h);if(f){let u=f.llamadas||{calls:0,sondeo:0,trabajo:0,errores:0,tools:0,principales:[]};f.llamadas={calls:u.calls+l.calls,sondeo:u.sondeo+l.sondeo,trabajo:u.trabajo+l.trabajo,errores:u.errores+l.errores,tools:Math.max(u.tools,l.tools),principales:[...u.principales||[],a]};continue}let d=Ms(a),c=ko[d]?d:"";c||r++,s.push({id:a,principal:a,tipo:"actor",persona:c||zo,exacta:!!c,...l})}return s.sort((o,a)=>a.calls-o.calls||o.id.localeCompare(a.id)),{terminales:n||[],actores:s,sinDeclarar:r}}function Bu(n,t){let e=new Map,i=new Map,s=new Set((n||[]).map(r=>r.id));for(let[r,o]of Object.entries(ko))s.has(o)&&(e.set(r,{id:o,tipo:"terminal"}),Ms(r)===r&&i.set(r,{id:o,tipo:"terminal"}));for(let r of t||[])e.set(r.principal,{id:r.id,tipo:"actor"});return{directo:e,porPersona:i}}function uv(){let n=new Map,t=new Map;for(let[e,i]of Object.entries(ko))n.set(e,{id:i,tipo:"terminal"}),Ms(e)===e&&t.set(e,{id:i,tipo:"terminal"});return{directo:n,porPersona:t}}function zu(n,t){let e=String(n&&n.principal||"").toLowerCase();if(!e)return null;let i=t||uv(),s=i.directo.get(e);if(s)return{terminal:s.id,tipo:s.tipo,persona:Ms(e),principal:e,exacta:!0};let r=Ms(e),o=i.porPersona.get(r);return o?{terminal:o.id,tipo:o.tipo,persona:r,principal:e,exacta:!1}:null}function ku(n){let t=String(n&&n.kind||"").toLowerCase(),e=String(n&&n.outcome||"").toLowerCase(),i=Number(n&&n.ms);return{capa:t==="sondeo"?"sondeo":"trabajo",falla:e!==""&&e!=="ok",ms:Number.isFinite(i)&&i>=0?i:0,tool:String(n&&n.tool||"")}}var dv=n=>`${n.topic||""} ${n.gist||""}`.toUpperCase();function Du(n){for(let t of hv)if(n.includes(t))return t;return null}var Uu=/([A-ZÁÉÍÓÚÑ][A-ZÁÉÍÓÚÑ \-]{2,26})\s*(?:→|->)\s*\*{0,2}([A-ZÁÉÍÓÚÑ][A-ZÁÉÍÓÚÑ \-·]{2,40})/g;function Hu(n){let t=n&&n.neurons||[],e=new Map,i=new Map,s=0;for(let a of t){let l=dv(a),h=Ms(a.author);h||s++;let f=(a.gist||"").toUpperCase().replace(/^[^A-ZÁÉÍÓÚÑ]+/,"");for(let c of Ou){if(!l.includes(c))continue;let u=e.get(c);u||(u={id:c,notas:0,firmadas:0,calor:0,autores:new Map,firmas:new Map},e.set(c,u)),u.notas++,u.calor+=typeof a.heat=="number"&&a.heat>0?a.heat:0,f.startsWith(c)&&u.firmadas++,h&&(u.autores.set(h,(u.autores.get(h)||0)+1),f.startsWith(c)&&u.firmas.set(h,(u.firmas.get(h)||0)+1))}if(!l.includes("\u2192")&&!l.includes("->"))continue;Uu.lastIndex=0;let d;for(;(d=Uu.exec(l))!==null;){let c=Du(d[1]),u=Du(d[2]);if(!c||!u||c===u)continue;let g=c+">"+u;i.set(g,(i.get(g)||0)+1)}}let r=[...e.values()].map(a=>({id:a.id,notas:a.notas,firmas:a.firmadas,calor:a.calor,persona:Nu(a.firmas)||Nu(a.autores),autores:[...a.autores.entries()].sort((l,h)=>h[1]-l[1]).map(([l,h])=>({persona:l,notas:h}))})).sort((a,l)=>l.notas-a.notas||a.id.localeCompare(l.id)),o=[...i.entries()].map(([a,l])=>{let[h,f]=a.split(">");return{de:h,a:f,veces:l}}).sort((a,l)=>l.veces-a.veces||a.de.localeCompare(l.de)||a.a.localeCompare(l.a));return{terminales:r,despachos:o,sinAutor:s}}function Nu(n){let t="",e=-1;for(let i of[...n.keys()].sort()){let s=n.get(i);s>e&&(e=s,t=i)}return t}function Vu(n,t){let e=new Map,i=(s,r)=>{e.has(s)||e.set(s,[]),e.get(s).push(r)};for(let s of n||[])i(s.persona||"(sin autor)",{...s,tipo:"terminal"});for(let s of t||[])i(s.persona||zo,s);return[...e.entries()].map(([s,r])=>({persona:s,nodos:r,notas:r.reduce((o,a)=>o+(a.tipo==="terminal"?a.notas:0),0),llamadas:r.reduce((o,a)=>o+(a.tipo==="actor"?a.calls:a.llamadas?a.llamadas.calls:0),0)})).sort((s,r)=>s.persona===zo?1:r.persona===zo?-1:r.notas-s.notas||s.persona.localeCompare(r.persona))}var fv=[79,107,255],pv=[63,208,163],cr=[53,208,224],Ho=[fv,pv,[160,107,255],cr,[232,178,74]],Xu=n=>{let t=Ho[n%Ho.length];return`rgb(${t[0]},${t[1]},${t[2]})`},wc="rgb(138,153,255)",Ac=`rgb(${cr[0]},${cr[1]},${cr[2]})`,Gu=n=>()=>(n=n*1664525+1013904223>>>0)/4294967296,mv=n=>{let t=2166136261;for(let e=0;e<n.length;e++)t=Math.imul(t^n.charCodeAt(e),16777619);return(t>>>0)%1e3/1e3*6.2832},$n=(n,t)=>[n[0]+t[0],n[1]+t[1],n[2]+t[2]],Nn=(n,t)=>[n[0]*t,n[1]*t,n[2]*t],hr=n=>{let t=Math.hypot(n[0],n[1],n[2])||1;return[n[0]/t,n[1]/t,n[2]/t]},Wu=(n,t)=>[n[1]*t[2]-n[2]*t[1],n[2]*t[0]-n[0]*t[2],n[0]*t[1]-n[1]*t[0]],lr=(n,t,e)=>Math.max(t,Math.min(e,n));function gv(n,t,e){let i=Math.abs(n[1])>.9?[1,0,0]:[0,1,0],s=hr(Wu(n,i)),r=hr(Wu(n,s)),o=e()*Math.PI*2,a=$n(Nn(s,Math.cos(o)),Nn(r,Math.sin(o)));return hr($n(Nn(n,Math.cos(t)),Nn(a,Math.sin(t))))}function qu(n){let t=n.getContext("2d"),e={yaw:.2,pitch:-.24,dist:1180,f:2800,zoom:1},i=.4,s=40,r=2,o=0,a=0,l=null,h=[],f=[],d=new Map,c=[0,0,0],u=600,g=300,y=0,p=0,m=0,w=0,_=null,A=!1,I="rotar",C=0,E=0,T=!1,H=0,x=1,S=null,O=[],W=!matchMedia("(prefers-reduced-motion: reduce)").matches,q=.85,et=.22,F=6,it=0,Z=0;function gt(){let j=Math.min(devicePixelRatio||1,2);o=n.clientWidth,a=n.clientHeight,n.width=Math.round(o*j),n.height=Math.round(a*j),t.setTransform(j,0,0,j,0,0),J()}function _t(){let lt=20,ut=20,Rt=o-20,B=a-20,M=z=>{let nt=document.querySelector(z);if(!nt)return null;let Q=nt.getBoundingClientRect();return Q.width>1&&Q.height>1?Q:null},v=M("#hud .rail.l");v&&(lt=Math.max(lt,v.right+20));let R=M("#hud .rail.r");R&&(Rt=Math.min(Rt,R.left-20));let k=M("#hud .hdr");k&&(ut=Math.max(ut,k.bottom+20));let K=M("#hud .center");return K&&(B=Math.min(B,K.top-20)),Rt-lt<120||B-ut<120?{x0:0,y0:0,x1:o,y1:a}:{x0:lt,y0:ut,x1:Rt,y1:B}}let St=.5,Gt=200,Wt=200;function J(){if(!o||!a)return;let j=_t();y=(j.x0+j.x1)/2,p=(j.y0+j.y1)/2,Gt=Math.max((j.x1-j.x0)/2*.96,60),Wt=Math.max((j.y1-j.y0)/2*.96,60);let lt=g*Math.cos(St)+u*Math.sin(St);e.dist=Math.max(240,u+e.f*u/Gt,u+e.f*lt/Wt)}let st=()=>e.f/(e.dist/e.zoom);function Mt(j,lt){if(l=j,h=[],f=[],d=new Map,m=0,w=0,e.zoom=1,S=null,!lt.length)return;let ut=X=>88+36*Math.sqrt(X.nodos.length),Rt=92,B=0,M=lt.map((X,ht)=>(ht>0&&(B+=ut(lt[ht-1])+Rt+ut(X)),[B,0,0])),v=M.length?(M[0][0]+M[M.length-1][0])/2:0;for(let X of M)X[0]-=v;x=1,lt.forEach((X,ht)=>{let tt=Ho[ht%Ho.length],ct=M[ht],At=ut(X);X.nodos.forEach((rt,ot)=>{let Lt=ot+.5,wt=Math.acos(1-2*Lt/X.nodos.length),Ft=Math.PI*(1+Math.sqrt(5))*Lt,L=rt.id.toLowerCase()===X.persona?At*.18:At,ft=rt.tipo==="actor",Y=!ft&&Number.isFinite(rt.calor)?rt.calor:0;Y>x&&(x=Y);let $={id:rt.id,tipo:ft?"actor":"terminal",notas:ft?0:rt.notas,calor:Y,firmas:rt.firmas||0,persona:X.persona,col:tt,llamadas:ft?{calls:rt.calls,sondeo:rt.sondeo,trabajo:rt.trabajo,tools:rt.tools,proyecto:rt.proyecto}:rt.llamadas||null,exacta:ft?rt.exacta!==!1:!0,r:ft?Math.max(3.4,Math.log10(1+(rt.calls||0))*3.4):Math.max(4.2,Math.sqrt(rt.notas)*1.15),fase:mv(rt.id),seg:[],finNivel:[0],alcance:0,alcanceRama:1,pulsos:[],pos:[ct[0]+Math.cos(Ft)*Math.sin(wt)*L,ct[1]+Math.cos(wt)*L*.78,ct[2]+Math.sin(Ft)*Math.sin(wt)*L]};f.push($),d.set(rt.id,$)})});let R=7;for(let X of f){let ht=Gu(R+=977);if(X.tipo==="actor"){Nt(X);continue}let tt=Math.max(4,Math.min(7,Math.round(Math.log(X.notas+1)*1.6)));X.profBase=X.notas>150?5:X.notas>55?4:3;let ct=X.r*(X.notas>120?2.5:2.9);for(let rt=0;rt<tt;rt++){let ot=rt+.5,Lt=Math.acos(1-2*ot/tt),wt=Math.PI*(1+Math.sqrt(5))*ot,Ft=hr([Math.cos(wt)*Math.sin(Lt),Math.cos(Lt),Math.sin(wt)*Math.sin(Lt)]);xt($n(X.pos,Nn(Ft,X.r*.85)),Ft,ct,Math.max(1.5,X.r*.3),X.profBase+r,0,X,ht,X.r*.85)}X.seg.sort((rt,ot)=>rt.nivel-ot.nivel);let At=X.profBase+r;X.finNivel=new Array(At+1).fill(0);for(let rt of X.seg)for(let ot=rt.nivel;ot<=At;ot++)X.finNivel[ot]++;for(let rt of X.seg){let ot=Math.hypot(rt.b[0]-X.pos[0],rt.b[1]-X.pos[1],rt.b[2]-X.pos[2]);ot>X.alcance&&(X.alcance=ot)}}for(let X of l.despachos){let ht=d.get(X.de),tt=d.get(X.a);if(!ht||!tt)continue;let ct=Gu(X.de.length*131+X.a.length*17+X.veces),At=Nn($n(ht.pos,tt.pos),.5),rt=$n(At,[(ct()-.5)*150,-70-X.veces*4.5-ct()*60,(ct()-.5)*150]),ot=[];for(let Lt=0;Lt<=26;Lt++){let wt=Lt/26,Ft=1-wt;ot.push([Ft*Ft*ht.pos[0]+2*Ft*wt*rt[0]+wt*wt*tt.pos[0],Ft*Ft*ht.pos[1]+2*Ft*wt*rt[1]+wt*wt*tt.pos[1],Ft*Ft*ht.pos[2]+2*Ft*wt*rt[2]+wt*wt*tt.pos[2]])}h.push({de:X.de,a:X.a,veces:X.veces,P:ot,cruza:ht.persona!==tt.persona,Q:new Array(ot.length)})}let k=[1e9,1e9,1e9],K=[-1e9,-1e9,-1e9];for(let X of f)for(let ht=0;ht<3;ht++)X.pos[ht]<k[ht]&&(k[ht]=X.pos[ht]),X.pos[ht]>K[ht]&&(K[ht]=X.pos[ht]);c=[(k[0]+K[0])/2,(k[1]+K[1])/2,(k[2]+K[2])/2];let z=0,nt=0,Q=46;for(let X of f){let ht=Math.hypot(X.pos[0]-c[0],X.pos[2]-c[2]);ht+Q>z&&(z=ht+Q);let tt=Math.abs(X.pos[1]-c[1]);tt+Q>nt&&(nt=tt+Q)}u=Math.max(120,z),g=Math.max(80,nt),J()}function xt(j,lt,ut,Rt,B,M,v,R,k){if(B<=0||Rt<.06||v.seg.length>9e3)return;let K=R()<.28?3:2;for(let z=0;z<K;z++){let nt=gv(lt,.38+R()*.42,R),Q=ut*(.66+R()*.16),X=$n(j,Nn(nt,Q)),ht=k+Q;ht>v.alcanceRama&&(v.alcanceRama=ht),v.seg.push({a:j,b:X,w:Rt,nivel:M,dist:ht}),xt(X,nt,Q,Rt*.6,B-1,M+1,v,R,ht)}}function Nt(j){let lt=Math.max(2,Math.min(12,j.llamadas&&j.llamadas.tools||2)),ut=j.r*2.4;for(let Rt=0;Rt<lt;Rt++){let B=Rt+.5,M=Math.acos(1-2*B/lt),v=Math.PI*(1+Math.sqrt(5))*B,R=hr([Math.cos(v)*Math.sin(M),Math.cos(M),Math.sin(v)*Math.sin(M)]),k=$n(j.pos,Nn(R,j.r*.9)),K=$n(j.pos,Nn(R,j.r*.9+ut));j.seg.push({a:k,b:K,w:Math.max(.9,j.r*.22),nivel:0,dist:j.r*.9+ut})}j.alcanceRama=j.r*.9+ut,j.alcance=j.alcanceRama,j.profBase=0,j.finNivel=new Array(r+1).fill(j.seg.length)}function It(j){it++;let lt=j&&j.terminal?d.get(j.terminal):null;return lt?(lt.pulsos.length>=F&&lt.pulsos.shift(),lt.pulsos.push({t0:H,capa:j.capa==="sondeo"?"sondeo":"trabajo",falla:!!j.falla,grosor:1+Math.min(1.6,Math.log10(1+Math.max(0,Number(j.ms)||0))*.42),exacta:j.exacta!==!1}),!0):(Z++,!1)}function kt(j){let lt=j[0]-c[0],ut=j[1]-c[1],Rt=j[2]-c[2],B=Math.cos(e.yaw),M=Math.sin(e.yaw),v=lt*B-Rt*M,R=lt*M+Rt*B,k=ut,K=Math.cos(e.pitch),z=Math.sin(e.pitch),nt=k*K-R*z,X=k*z+R*K+e.dist/e.zoom,ht=e.f/Math.max(X,40);return{x:y+m+v*ht,y:p+w+nt*ht,s:ht,z:X}}function Zt(){if(t.clearRect(0,0,o,a),!f.length){t.fillStyle="rgba(154,157,171,.7)",t.font="13px 'JetBrains Mono', ui-monospace, monospace",t.textAlign="center",t.fillText("todav\xEDa no hay despachos entre terminales en esta memoria",y||o/2,p||a/2);return}O.length=0;let j=e.dist/e.zoom-u,lt=Math.max(2*u,1),ut=B=>lr((B-j)/lt,0,1),Rt=lr(Math.round(Math.log2(Math.max(e.zoom,1))),0,r);for(let B of f){let M=kt(B.pos),v=B.alcance*M.s+40;if(M.x+v<0||M.x-v>o||M.y+v<0||M.y-v>a){B._sx=null;continue}B._P=M;let R=B.pulsos;for(;R.length&&H-R[0].t0>q;)R.shift();let k=[];for(let z=0;z<R.length;z++){let nt=R[z],Q=(H-nt.t0)/q;k.push({en:Q*B.alcanceRama,radio:et*B.alcanceRama,fuerza:(nt.capa==="trabajo"?1:.3)*(1-Q*Q)*(nt.exacta?1:.65),falla:nt.falla,grosor:nt.grosor})}B._flash=0;for(let z of k)z.en<z.radio&&(B._flash=Math.max(B._flash,z.fuerza*(1-z.en/z.radio)));O.push({z:M.z,t:2,nd:B,P:M});let K=B.finNivel[Math.min(B.finNivel.length-1,B.profBase+Rt)];for(let z=0;z<K;z++){let nt=B.seg[z],Q=kt(nt.a),X=kt(nt.b);if(Math.abs(Q.x-X.x)+Math.abs(Q.y-X.y)<.8)continue;let ht=0,tt=1,ct=!1;for(let At=0;At<k.length;At++){let rt=k[At],ot=Math.abs(nt.dist-rt.en)/rt.radio;if(ot>=1)continue;let Lt=(1-ot)*(1-ot);ht+=Lt*rt.fuerza,Lt>.35&&(tt=Math.max(tt,rt.grosor),ct=ct||rt.falla)}O.push({z:(Q.z+X.z)/2,t:0,A:Q,B:X,w:nt.w,col:B.col,nodo:B.id,carga:ht>.01?Math.min(1.6,ht):0,gordo:tt,falla:ct})}}for(let B of h){for(let M=0;M<B.P.length;M++)B.Q[M]=kt(B.P[M]);for(let M=0;M<B.Q.length-1;M++){let v=B.Q[M],R=B.Q[M+1];O.push({z:(v.z+R.z)/2,t:1,A:v,B:R,ax:B,ult:M===B.Q.length-2})}}O.sort((B,M)=>M.z-B.z);for(let B of O)if(B.t===0){let M=_&&_!==B.nodo,v=1-.78*ut(B.A.z),R=B.col;if(t.lineCap="round",t.strokeStyle=`rgba(${R[0]},${R[1]},${R[2]},${((M?.055:.3)*v).toFixed(3)})`,t.lineWidth=Math.max(.35,B.w*B.A.s*1.15),t.beginPath(),t.moveTo(B.A.x,B.A.y),t.lineTo(B.B.x,B.B.y),t.stroke(),B.carga>0&&!M){let k=Math.min(1,B.carga),K=Math.round(R[0]+(255-R[0])*k*.9),z=Math.round(R[1]+(255-R[1])*k*.9),nt=Math.round(R[2]+(255-R[2])*k*.75),Q=B.falla?[245,196,81]:[K,z,nt],X=Math.max(.6,B.w*B.A.s*1.15),ht=t.globalCompositeOperation;t.globalCompositeOperation="lighter",t.strokeStyle=`rgba(${Q[0]},${Q[1]},${Q[2]},${(.2*k*v).toFixed(3)})`,t.lineWidth=Math.max(3.2,X*B.gordo*5.5),t.beginPath(),t.moveTo(B.A.x,B.A.y),t.lineTo(B.B.x,B.B.y),t.stroke(),t.strokeStyle=`rgba(${Q[0]},${Q[1]},${Q[2]},${(Math.min(1,B.carga)*v).toFixed(3)})`,t.lineWidth=Math.max(1.5,X*B.gordo*2.1),t.beginPath(),t.moveTo(B.A.x,B.A.y),t.lineTo(B.B.x,B.B.y),t.stroke(),t.globalCompositeOperation=ht}}else if(B.t===1){let M=_&&(_===B.ax.de||_===B.ax.a),v=_&&!M,R=1-.6*ut(B.A.z),k=B.ax.cruza?cr:[138,153,255],K=M?.62:v?.04:.27;if(t.strokeStyle=`rgba(${k[0]},${k[1]},${k[2]},${(K*R).toFixed(3)})`,t.lineWidth=Math.max(.5,(.7+B.ax.veces*.16)*B.A.s*(M?1.5:1)),t.beginPath(),t.moveTo(B.A.x,B.A.y),t.lineTo(B.B.x,B.B.y),t.stroke(),B.ult){let z=Math.atan2(B.B.y-B.A.y,B.B.x-B.A.x),nt=Math.max(4,7*B.B.s*(M?1.5:1));t.fillStyle=`rgba(${k[0]},${k[1]},${k[2]},${((K+.14)*R).toFixed(3)})`,t.beginPath(),t.moveTo(B.B.x,B.B.y),t.lineTo(B.B.x-Math.cos(z-.42)*nt,B.B.y-Math.sin(z-.42)*nt),t.lineTo(B.B.x-Math.cos(z+.42)*nt,B.B.y-Math.sin(z+.42)*nt),t.closePath(),t.fill()}}else{let M=B.nd,v=B.P,R=_&&_!==M.id,k=Math.log(1+M.calor)/Math.log(1+x),K=1+.17*k*Math.sin(H*1.9+M.fase),z=M._flash||0,nt=Math.max(2.2,M.r*v.s*1.5)*K*(1+.9*z),Q=M.col,X=nt+Math.min(nt*1.6,38),ht=t.createRadialGradient(v.x,v.y,0,v.x,v.y,X);if(ht.addColorStop(0,`rgba(${Q[0]},${Q[1]},${Q[2]},${R?.1:.22+.14*k*(K-1)/.17})`),ht.addColorStop(1,`rgba(${Q[0]},${Q[1]},${Q[2]},0)`),t.fillStyle=ht,t.beginPath(),t.arc(v.x,v.y,X,0,6.2832),t.fill(),M.tipo==="actor"?(t.strokeStyle=`rgba(${Q[0]},${Q[1]},${Q[2]},${R?.3:.95})`,t.lineWidth=Math.max(1.2,nt*.34),M.exacta===!1&&t.setLineDash([Math.max(2,nt*.5),Math.max(2,nt*.42)]),t.beginPath(),t.arc(v.x,v.y,nt,0,6.2832),t.stroke(),t.setLineDash([]),z>.01&&(t.fillStyle=`rgba(255,255,255,${(.75*z).toFixed(3)})`,t.beginPath(),t.arc(v.x,v.y,nt*.52*z,0,6.2832),t.fill())):(t.fillStyle=`rgba(${Q[0]},${Q[1]},${Q[2]},${R?.28:.95})`,t.beginPath(),t.arc(v.x,v.y,nt,0,6.2832),t.fill(),t.fillStyle=`rgba(255,255,255,${R?.06:Math.min(1,.22+.78*z).toFixed(3)})`,t.beginPath(),t.arc(v.x,v.y,nt*(.42+.18*z),0,6.2832),t.fill()),M._sx=v.x,M._sy=v.y,M._sr=nt,!R){let tt=M.tipo==="actor"?M.llamadas&&M.llamadas.calls>=1e3:M.notas>=60;t.fillStyle=_===M.id?"#ECEAE3":tt?"rgba(236,234,227,.62)":"rgba(236,234,227,.34)";let ct=M.tipo==="actor";t.font=`${Math.max(8,Math.min(ct?12:16,(ct?9:11)*v.s*1.3))}px 'JetBrains Mono', ui-monospace, monospace`,t.textAlign="center";let At=ct&&M.persona&&M.id.startsWith(M.persona+"-")?M.id.slice(M.persona.length+1):M.id;t.fillText(At.toLowerCase(),v.x,v.y+nt+(ct?11:14))}}}let Vt=j=>{let lt=n.getBoundingClientRect(),ut=j.clientX-lt.left,Rt=j.clientY-lt.top,B=null,M=1e9;for(let v of f){if(v._sx==null)continue;let R=Math.hypot(ut-v._sx,Rt-v._sy);R<Math.max(16,v._sr*1.9)&&R<M&&(M=R,B=v)}return B};n.addEventListener("pointerdown",j=>{A=!0,I=j.shiftKey||j.button===1?"pan":"rotar",C=j.clientX,E=j.clientY,S=null,P();try{n.setPointerCapture(j.pointerId)}catch{}}),n.addEventListener("pointerup",()=>{A=!1}),n.addEventListener("pointermove",j=>{if(A){I==="pan"?(m+=j.clientX-C,w+=j.clientY-E):(e.yaw+=(j.clientX-C)*.006,e.pitch=lr(e.pitch+(j.clientY-E)*.005,-1.2,1.2)),C=j.clientX,E=j.clientY;return}let lt=Vt(j),ut=lt?lt.id:null;ut!==_&&(_=ut,fe&&fe(lt,l,j.clientX,j.clientY))}),n.addEventListener("pointerleave",()=>{P()});function P(){_!==null&&(_=null,fe&&fe(null,l))}n.addEventListener("wheel",j=>{j.preventDefault();let lt=n.getBoundingClientRect(),ut=j.clientX-lt.left,Rt=j.clientY-lt.top,B=st();e.zoom=lr(e.zoom*(j.deltaY>0?1/1.12:1.12),i,s);let M=st()/B;m=ut-y-(ut-y-m)*M,w=Rt-p-(Rt-p-w)*M,S=null,P()},{passive:!1}),n.addEventListener("dblclick",j=>{let lt=Vt(j);if(!lt){S={zoom:1,panx:0,pany:0,t:0};return}let ut=Math.max(lt.alcance,lt.r*3),Rt=lr(.62*Math.min(Gt,Wt)*e.dist/(e.f*ut),1,s),B=e.zoom,M=m,v=w;e.zoom=Rt,m=0,w=0;let R=kt(lt.pos),k={zoom:Rt,panx:y-R.x,pany:p-R.y,t:0};e.zoom=B,m=M,w=v,S=k});let fe=null;return addEventListener("resize",()=>{T&&gt()}),{cargar(j,lt){Mt(j,lt)},activar(j){T=j,j&&gt()},activa(){return T},frame(j){if(T){if(j&&W&&(H+=1/60,A||_||e.zoom>1.35||m||w||(e.yaw+=.0018)),S){S.t=Math.min(1,S.t+.075);let lt=1-Math.pow(1-S.t,3);e.zoom+=(S.zoom-e.zoom)*lt*.5,m+=(S.panx-m)*lt*.5,w+=(S.pany-w)*lt*.5,S.t>=1&&(e.zoom=S.zoom,m=S.panx,w=S.pany,S=null)}Zt()}},onFoco(j){fe=j},pulsar:It,cuentaPulsos(){return{vistos:it,sinNeurona:Z}}}}function Yu(n){return n>2e3?55:n>500?90:n>180?150:230}function Ic(n,t,e){return e?t<=0?0:Math.min(Yu(n),4+t*2):Yu(n)}var xv=700,vv=.7*.7,Re=0,Fn,ti,Ai,Ei,Ti,fn,Ci,Ri,Pi,Ii,Tc=0;function ju(n){n<=Re||(Re=Math.max(n,Re*2,1024),Fn=new Int32Array(Re*8),ti=new Int32Array(Re),Ai=new Float64Array(Re),Ei=new Float64Array(Re),Ti=new Float64Array(Re),fn=new Int32Array(Re),Ci=new Float64Array(Re),Ri=new Float64Array(Re),Pi=new Float64Array(Re),Ii=new Float64Array(Re))}function Cc(n,t,e,i){Tc>=Re&&_v();let s=Tc++;return Fn.fill(-1,s*8,s*8+8),ti[s]=0,Ai[s]=Ei[s]=Ti[s]=0,fn[s]=-1,Ci[s]=i,Ri[s]=n,Pi[s]=t,Ii[s]=e,s}function _v(){let n={chi:Fn,cnt:ti,cx:Ai,cy:Ei,cz:Ti,bod:fn,h:Ci,ox:Ri,oy:Pi,oz:Ii,n:Re};Re=n.n,ju(n.n*2||1024),Fn.set(n.chi),ti.set(n.cnt),Ai.set(n.cx),Ei.set(n.cy),Ti.set(n.cz),fn.set(n.bod),Ci.set(n.h),Ri.set(n.ox),Pi.set(n.oy),Ii.set(n.oz)}function Zu(n,t,e,i){return(t>Ri[n]?1:0)|(e>Pi[n]?2:0)|(i>Ii[n]?4:0)}function yv(n,t,e,i,s){let r=n;for(let o=0;o<64;o++){if(ti[r]++,Ai[r]+=e,Ei[r]+=i,Ti[r]+=s,ti[r]===1){fn[r]=t;return}if(fn[r]>=0){let f=fn[r];fn[r]=-1;let d=ke[f].x,c=ke[f].y,u=ke[f].z,g=Zu(r,d,c,u),y=Ci[r]*.5,p=Fn[r*8+g];p<0&&(p=Cc(Ri[r]+(g&1?y:-y),Pi[r]+(g&2?y:-y),Ii[r]+(g&4?y:-y),y),Fn[r*8+g]=p),ti[p]++,Ai[p]+=d,Ei[p]+=c,Ti[p]+=u,fn[p]=f}let a=Zu(r,e,i,s),l=Ci[r]*.5,h=Fn[r*8+a];h<0&&(h=Cc(Ri[r]+(a&1?l:-l),Pi[r]+(a&2?l:-l),Ii[r]+(a&4?l:-l),l),Fn[r*8+a]=h),r=h}}var Vo=new Int32Array(16384);function Mv(n,t,e,i,s,r,o,a){let l=0;Vo[l++]=n;let h=0,f=0,d=0;for(;l>0;){let c=Vo[--l],u=ti[c];if(u===0)continue;let g=Ai[c]/u,y=Ei[c]/u,p=Ti[c]/u,m=e-g,w=i-y,_=s-p,A=m*m+w*w+_*_,I=fn[c]>=0;if(I&&fn[c]===t)continue;let C=Ci[c]*2;if(I||C*C<vv*A){if(A>r||A<.001)continue;let E=o*u/A,T=Math.sqrt(A);h+=m/T*E,f+=w/T*E,d+=_/T*E}else for(let E=0;E<8;E++){let T=Fn[c*8+E];T>=0&&l<Vo.length&&(Vo[l++]=T)}}a[0]=h,a[1]=f,a[2]=d}var Go=new Float64Array(3),ke=[],Ku=[],Rc=()=>({rx:118,ry:94,rz:87}),wi=0,Ju=0,Qu=0,$u=0,Pc=!1,Sv=.09,bv=.0042,wv=.86,Av=256,Ss=0,On=0,Ec=0;function Ev(n,t,e,i){let s=n.x*n.x/(t*t)+n.y*n.y/(e*e)+n.z*n.z/(i*i);if(s>1){let r=1/Math.sqrt(s);n.x*=r,n.y*=r,n.z*=r,n.vx*=.4,n.vy*=.4,n.vz*=.4}}function td(n,t,e,i){ke=n,Ku=t,Rc=e;let s=ke.length;if(!s){wi=0;return}let r=Rc().rx,o=r*.85;Ju=o*o,Qu=r*r*.06,$u=r*.044,Pc=s>=xv,Pc&&ju(Math.max(1024,s*4)),wi=i,Ss=0,On=0}function Lc(){return wi}function ed(n){if(wi<=0)return{trabajo:!1,termino:!1};let t=performance.now();do if(Tv(t,n))wi--;else break;while(wi>0&&performance.now()-t<n);return{trabajo:!0,termino:wi===0}}function Tv(n,t){let e=ke.length;if(!e)return!0;let i=Ju,s=Qu,r=$u,o=Sv,a=bv,l=wv,h=Pc;if(Ss===0){if(h){let d=1;for(let c of ke){let u=Math.max(Math.abs(c.x),Math.abs(c.y),Math.abs(c.z));u>d&&(d=u)}Tc=0,Ec=Cc(0,0,0,d*1.05);for(let c=0;c<e;c++){let u=ke[c];yv(Ec,c,u.x,u.y,u.z)}}Ss=1,On=0}if(Ss===1){if(h)for(;On<e;){let d=Math.min(e,On+Av);for(let c=On;c<d;c++){let u=ke[c];Mv(Ec,c,u.x,u.y,u.z,i,s,Go),u.vx+=Go[0]*.5-u.x*a,u.vy+=Go[1]*.5-u.y*a,u.vz+=Go[2]*.5-u.z*a}if(On=d,On<e&&performance.now()-n>=t)return!1}else{for(let d=0;d<e;d++){let c=ke[d],u=0,g=0,y=0;for(let p=d+1;p<e;p++){let m=ke[p],w=c.x-m.x,_=c.y-m.y,A=c.z-m.z,I=w*w+_*_+A*A;if(I>i||I<.001)continue;let C=s/I,E=Math.sqrt(I),T=w/E,H=_/E,x=A/E;u+=T*C,g+=H*C,y+=x*C,m.vx-=T*C*.5,m.vy-=H*C*.5,m.vz-=x*C*.5}c.vx+=u*.5-c.x*a,c.vy+=g*.5-c.y*a,c.vz+=y*.5-c.z*a}On=e}Ss=2}for(let d of Ku){let c=ke[d.a],u=ke[d.b],g=u.x-c.x,y=u.y-c.y,p=u.z-c.z,m=Math.hypot(g,y,p)||1,w=(m-r)*o,_=g/m,A=y/m,I=p/m;c.vx+=_*w,c.vy+=A*w,c.vz+=I*w,u.vx-=_*w,u.vy-=A*w,u.vz-=I*w}let f=Rc();for(let d of ke){d.vx*=l,d.vy*=l,d.vz*=l;let c=d.vx*d.vx+d.vy*d.vy+d.vz*d.vz;if(c>1600){let u=40/Math.sqrt(c);d.vx*=u,d.vy*=u,d.vz*=u}d.x+=d.vx,d.y+=d.vy,d.z+=d.vz,Ev(d,f.rx,f.ry,f.rz)}return Ss=0,On=0,!0}var Qo=["#2dd4bf","#a78bfa","#fbbf24","#4ade80","#38bdf8","#f472b6","#fb923c","#f87171","#a3e635","#22d3ee","#e879f9","#facc15"];var kc=["#7f9cc9","#43e08b","#31c9ff","#f5c451"],dd=kc[0],Li={CALLS:"#38bdf8",IMPORTS:"#a78bfa",CONTAINS:"#5b6b86"},fd=n=>n&&n.kind&&Li[n.kind]||dd,Oi=new Map,Bn=[],pd=n=>Oi.get(n)||"#64748b";function md(n){let t=2166136261;for(let e=0;e<n.length;e++)t^=n.charCodeAt(e),t=Math.imul(t,16777619);return(t>>>0)%1e3/1e3}var Cv=118,Zc=1,ii=118,Us=94,Ns=87;function gd(){ii=Cv*Zc,Us=ii,Ns=ii}var Hc="memory",Rv=()=>({rx:ii,ry:Us,rz:Ns});function xd(n,t){td(Pt,Ke,Rv,n),Lc()>0&&(Hc=t,oa[t]=!1)}function Pv(n){let t=ed(n);return t.trabajo?(t.termino&&(gr[Hc]=new Map(Pt.map(e=>[e.id,{x:e.x,y:e.y,z:e.z,ph:e.ph}])),oa[Hc]=!0),!0):!1}var Iv=(n,t,e)=>n*n/(ii*ii)+t*t/(Us*Us)+e*e/(Ns*Ns)<=1;function vd(){for(let n=0;n<80;n++){let t=(Math.random()*2-1)*ii,e=(Math.random()*2-1)*Us,i=(Math.random()*2-1)*Ns;if(Iv(t,e,i))return{x:t,y:e,z:i}}return{x:0,y:0,z:0}}var Pt=[],Ke=[],gr={memory:new Map,code:new Map},oa={memory:!1,code:!1},bs=new Map,Vc=new Set,ei=0,pr=new Float32Array(0),Rs=new Float32Array(0),$o=new Int8Array(0),yn=!0,Os=!1,_e="memory",si=qu(document.getElementById("personas")),xr=[],Ze=null,pn=null,_d=null,ws=null;function Lv(n){let t=gr.memory,e=n.neurons||[];Zc=Math.max(.85,Math.min(2.2,Math.cbrt((e.length||1)/240))),gd();let s={};e.forEach(u=>s[u.domain]=(s[u.domain]||0)+1);let r=Object.keys(s).sort((u,g)=>s[g]-s[u]||u.localeCompare(g));Oi=new Map,Bn=[];let o=new Map;r.forEach((u,g)=>{let y=Qo[g%Qo.length];Oi.set(u,y),o.set(u,g);let p=g+.5,m=Math.acos(1-2*p/Math.max(r.length,1)),w=Math.PI*(1+Math.sqrt(5))*p;Bn.push({name:u,color:y,count:s[u],ax:Math.cos(w)*Math.sin(m)*ii*.52,ay:Math.cos(m)*Us*.52,az:Math.sin(w)*Math.sin(m)*Ns*.52})});let a=new Set(t.keys());Pt=e.map(u=>{let g=t.get(u.id),y=g?{x:g.x,y:g.y,z:g.z}:vd(),p=Math.max(.9,Math.min(6,.9+Math.sqrt(Math.max(u.importance,0))*.72+Math.log(1+(u.heat||0))*.38)),m=Math.max(.1,Math.min(1,1-(u.recency_days||0)/45));return{...u,x:y.x,y:y.y,z:y.z,vx:0,vy:0,vz:0,r:p,rec:m,col:pd(u.domain),ph:g&&g.ph!=null?g.ph:Math.random()*6.283,phx:Math.random()*6.283,phz:Math.random()*6.283,di:o.has(u.domain)?o.get(u.domain):-1,act:0,ak:0,adj:[],_new:!g}});let l=new Map(Pt.map((u,g)=>[u.id,g]));Ke=(n.synapses||[]).filter(u=>l.has(u.source)&&l.has(u.target)).map(u=>{let g=md(u.source+">"+u.target);return{...u,a:l.get(u.source),b:l.get(u.target),off:g}}),Pt.forEach(u=>{u.adj=[],u.deg=0});for(let u of Ke){let g=.35+(u.confidence||0)*.55;Pt[u.a].adj.push({j:u.b,w:g}),Pt[u.b].adj.push({j:u.a,w:g}),Pt[u.a].deg++,Pt[u.b].deg++}if(pr=new Float32Array(Pt.length),Rs=new Float32Array(Pt.length),$o=new Int8Array(Pt.length),bs.size===0)ei=0;else{let u=0;for(let g of Pt){let y=bs.get(g.id);y?(g.heat>y.heat||y.rec!=null&&g.recency_days!=null&&g.recency_days<y.rec-4e-4)&&(g.act=1,g.ak=2,u++):g.age_days!=null&&g.age_days<.02&&(g.act=1,g.ak=1,u++)}for(let g of Ke)!Vc.has(g.source+"|"+g.target)&&bs.has(g.source)&&bs.has(g.target)&&(Pt[g.a].act=1,Pt[g.a].ak=3,Pt[g.b].act=1,Pt[g.b].ak=3,u++);u>0&&(ei=Math.min(1,ei+.5+u*.2),Wc=!0)}bs=new Map(Pt.map(u=>[u.id,{heat:u.heat,rec:u.recency_days}])),Vc=new Set(Ke.map(u=>u.source+"|"+u.target));let f=Pt.reduce((u,g)=>u+(g._new?1:0),0),d=f>0||Pt.length!==a.size;gr.memory=new Map(Pt.map(u=>[u.id,{x:u.x,y:u.y,z:u.z,ph:u.ph}]));let c=Ic(Pt.length,f,oa.memory);c>0?(xd(c,"memory"),Os=!0):d&&(Os=!0)}function Dv(n){let t=gr.code,e=n.nodes||[];Zc=Math.max(.85,Math.min(2.2,Math.cbrt((e.length||1)/240))),gd();let s={};e.forEach(c=>s[c.module]=(s[c.module]||0)+1);let r=Object.keys(s).sort((c,u)=>s[u]-s[c]||c.localeCompare(u));Oi=new Map,Bn=[];let o=new Map;r.forEach((c,u)=>{let g=Qo[u%Qo.length];Oi.set(c,g),o.set(c,u),Bn.push({name:c,color:g,count:s[c]})});let a=new Set(t.keys());Pt=e.map(c=>{let u=t.get(c.key),g=u?{x:u.x,y:u.y,z:u.z}:vd(),y=Math.max(.9,Math.min(6,.9+Math.sqrt(Math.max(c.degree||0,0))*.62));return{id:c.key,topic:c.name||c.key,domain:c.module,mem_type:c.kind,heat:c.degree||0,path:c.path,line:c.line,gist:c.path?c.path+(c.line?":"+c.line:""):c.kind||"",_code:!0,_exp:void 0,x:g.x,y:g.y,z:g.z,vx:0,vy:0,vz:0,r:y,rec:1,col:pd(c.module),ph:u&&u.ph!=null?u.ph:Math.random()*6.283,phx:Math.random()*6.283,phz:Math.random()*6.283,di:o.has(c.module)?o.get(c.module):-1,act:0,ak:0,adj:[],_new:!u}});let l=new Map(Pt.map((c,u)=>[c.id,u]));Ke=(n.edges||[]).filter(c=>l.has(c.source)&&l.has(c.target)).map(c=>{let u=md(c.source+">"+c.target);return{...c,a:l.get(c.source),b:l.get(c.target),off:u}}),Pt.forEach(c=>{c.adj=[],c.deg=0});for(let c of Ke){let u=.35+(c.confidence||0)*.55;Pt[c.a].adj.push({j:c.b,w:u}),Pt[c.b].adj.push({j:c.a,w:u}),Pt[c.a].deg++,Pt[c.b].deg++}pr=new Float32Array(Pt.length),Rs=new Float32Array(Pt.length),$o=new Int8Array(Pt.length),bs=new Map,Vc=new Set,ei=0;let h=Pt.reduce((c,u)=>c+(u._new?1:0),0),f=h>0||Pt.length!==a.size;gr.code=new Map(Pt.map(c=>[c.id,{x:c.x,y:c.y,z:c.z,ph:c.ph}]));let d=Ic(Pt.length,h,oa.code);d>0?(xd(d,"code"),Os=!0):f&&(Os=!0)}function Uv(n){if(!ni)return;let t=Math.min(ae,Pt.length);if(n>=1){for(let e=0;e<t;e++){let i=Pt[e];ni[e]=i.x,Di[e]=i.y,Ui[e]=i.z}return}for(let e=0;e<t;e++){let i=Pt[e];ni[e]+=(i.x-ni[e])*n,Di[e]+=(i.y-Di[e])*n,Ui[e]+=(i.z-Ui[e])*n}}function Nv(){let n=Pt.length;if(n){pr.fill(0),Rs.fill(0);for(let t=0;t<n;t++){let e=Pt[t].act;if(e<.012)continue;let i=Pt[t].ak,s=Pt[t].adj;for(let r=0;r<s.length;r++){let o=s[r].j,a=e*s[r].w;pr[o]+=a,a>Rs[o]&&(Rs[o]=a,$o[o]=i)}}for(let t=0;t<n;t++){let e=Pt[t];e.act<.15&&Rs[t]>0&&(e.ak=$o[t]),e.act=Math.min(1,e.act*.93+pr[t]*.11)}}}var Ov=document.getElementById("brain"),Me=new uo({canvas:Ov,antialias:!0});Me.setPixelRatio(Math.min(devicePixelRatio,2));Me.info.autoReset=!1;var zs=new po;zs.fog=new fo(329485,.0016);var Mn=new De(58,innerWidth/innerHeight,1,6e3);Mn.position.set(0,20,340);var Fs=new Yn;zs.add(Fs);zs.add(new yo(2637919,.8));var yd=new sr(16773340,1.15);yd.position.set(-.5,.9,.7);zs.add(yd);var Md=new sr(7314431,.4);Md.position.set(.6,-.4,-.55);zs.add(Md);var Fv=new go(1,1),Sd=new xo({color:16777215,roughness:.4,metalness:0,flatShading:!0});Sd.onBeforeCompile=n=>{n.fragmentShader=n.fragmentShader.replace("#include <opaque_fragment>",`float _fres=pow(1.0-max(dot(normalize(normal),normalize(vViewPosition)),0.0),2.4);
  outgoingLight+=diffuseColor.rgb*_fres*1.5;
#include <opaque_fragment>`)};var Bv=new mo(1,1,1,6,1,!0),zv=["attribute vec3 aColor; attribute float aSpeed; attribute float aGlow; attribute float aBase;","varying vec2 vUv; varying vec3 vColor; varying float vSpeed; varying float vGlow; varying float vBase;","void main(){ vUv=uv; vColor=aColor; vSpeed=aSpeed; vGlow=aGlow; vBase=aBase;","  gl_Position=projectionMatrix*modelViewMatrix*instanceMatrix*vec4(position,1.0); }"].join(`
`),kv=["precision highp float;","uniform float uTime;","varying vec2 vUv; varying vec3 vColor; varying float vSpeed; varying float vGlow; varying float vBase;","float band(float y){ float p=fract(y); return smoothstep(0.0,0.06,p)*(1.0-smoothstep(0.06,0.36,p)); }","void main(){ float y=vUv.y-uTime*vSpeed; float pulse=band(y)+band(y+0.5); float i=vBase+pulse*vGlow; gl_FragColor=vec4(vColor*i,i); }"].join(`
`),sn=new yt;Me.getDrawingBufferSize(sn);var ks=new No(Me,new ue(sn.x,sn.y,{samples:4}));ks.setSize(sn.x,sn.y);ks.addPass(new Oo(zs,Mn));var Hv=new ys(new yt(sn.x,sn.y),.95,.7,.28);ks.addPass(Hv);ks.addPass(new Bo(sn.x,sn.y));var Bi=new Io(Mn,Me.domElement);Bi.rotateSpeed=2.4;Bi.zoomSpeed=1.3;Bi.panSpeed=.6;Bi.dynamicDampingFactor=.12;Bi.staticMoving=!1;var se=null,ae=0,ni,Di,Ui,nd,ta,ea,na,As,Es,Ts,Fi,Dc,Cs,je=null,Ps=null,nn=null,Is=null,Ls=null,Yo=null,Zo=null,jo=null,Gc=0,Wc=!1,Xc=-1,id=new $t,we=new Ot,sd=new Ot,Vv=new U,Gv=new U,Wv=new tn,rd=!1;function Xv(){se&&(Fs.remove(se),se.geometry.dispose(),se=null),je&&(Fs.remove(je),je.geometry.dispose(),je=null),Ps&&(Ps.dispose(),Ps=null),nn=Is=Ls=Yo=null}function qv(){if(Xv(),ae=Pt.length,!ae)return;ni=new Float32Array(ae),Di=new Float32Array(ae),Ui=new Float32Array(ae),nd=new Float32Array(ae),ta=new Float32Array(ae),ea=new Float32Array(ae),na=new Float32Array(ae),As=new Float32Array(ae),Es=new Float32Array(ae),Ts=new Float32Array(ae),Fi=new Float32Array(ae),Dc=[],Cs=Array.from({length:ae},()=>[]),se=new ir(Fv,Sd,ae),Zo=new Uint8Array(ae),Gc=1,Xc=-1;for(let t=0;t<ae;t++){let e=Pt[t];ni[t]=e.x,Di[t]=e.y,Ui[t]=e.z,nd[t]=e.r,ta[t]=e.phx,ea[t]=e.ph,na[t]=e.phz,Dc.push(new $e().setFromEuler(Wv.set(Math.random()*6.283,Math.random()*6.283,Math.random()*6.283))),id.compose(Vv.set(e.x,e.y,e.z),Dc[t],Gv.set(e.r,e.r,e.r)),se.setMatrixAt(t,id),se.setColorAt(t,we.set(e.col))}se.instanceMatrix.needsUpdate=!0,se.instanceColor&&(se.instanceColor.needsUpdate=!0),Fs.add(se);let n=Ke.length;if(n){nn=new Float32Array(n*3),Is=new Float32Array(n),Ls=new Float32Array(n),Yo=new Float32Array(n),jo=new Uint8Array(n);let t=Bv.clone();t.setAttribute("aColor",new Un(nn,3)),t.setAttribute("aSpeed",new Un(Is,1)),t.setAttribute("aGlow",new Un(Ls,1)),t.setAttribute("aBase",new Un(Yo,1)),Ps=new ie({uniforms:{uTime:{value:0}},vertexShader:zv,fragmentShader:kv,transparent:!0,blending:as,depthWrite:!1}),je=new ir(t,Ps,n),je.frustumCulled=!1;for(let e=0;e<n;e++){let i=Ke[e];Cs[i.a].push(i.b),Cs[i.b].push(i.a),i.__i=e,i.__r=.28+(i.confidence||0)*.5,we.set(fd(i)),nn[e*3]=we.r,nn[e*3+1]=we.g,nn[e*3+2]=we.b,Is[e]=.42+(i.confidence||0)*.5,Ls[e]=.55,Yo[e]=.06}Fs.add(je)}else for(let t of Ke)Cs[t.a].push(t.b),Cs[t.b].push(t.a);if(!rd&&ae){let t=0;for(let e of Pt){let i=Math.hypot(e.x,e.y,e.z);i>t&&(t=i)}Mn.position.set(0,20,Math.max(240,t*2.7)),rd=!0}Os=!1}var Bs=new So,Ni=new yt(-9,-9),ia=0,sa=0,ge=-1,Ko=-1,bd=new cn,ur=new U,od=new U,en=document.getElementById("tip");addEventListener("pointermove",n=>{Ni.x=n.clientX/innerWidth*2-1,Ni.y=-(n.clientY/innerHeight)*2+1,ia=n.clientX,sa=n.clientY});Me.domElement.addEventListener("pointerdown",n=>{if(!se)return;Ni.x=n.clientX/innerWidth*2-1,Ni.y=-(n.clientY/innerHeight)*2+1,Bs.setFromCamera(Ni,Mn);let t=Bs.intersectObject(se);if(t.length){ge=t[0].instanceId,Ko=n.pointerId;try{Me.domElement.setPointerCapture(n.pointerId)}catch{}Me.domElement.style.cursor="grabbing",Mn.getWorldDirection(od),bd.setFromNormalAndCoplanarPoint(od,t[0].point),Fi.fill(0);for(let e of Cs[ge])Fi[e]=.55;n.stopImmediatePropagation(),n.preventDefault()}},!0);function aa(){if(ge>=0){ge=-1,Me.domElement.style.cursor="",Fi&&Fi.fill(0);try{Ko>=0&&Me.domElement.releasePointerCapture(Ko)}catch{}Ko=-1}}Me.domElement.addEventListener("pointerup",aa,!0);Me.domElement.addEventListener("pointercancel",aa,!0);addEventListener("pointerup",aa);addEventListener("blur",aa);function Yv(){if(ge>=0||!se){en.classList.remove("on");return}Bs.setFromCamera(Ni,Mn);let n=Bs.intersectObject(se);if(n.length){let t=Pt[n[0].instanceId];if(en.querySelector(".tt").textContent=t.topic||t.domain,t._code){Zv(t);let o=de(t.gist||"(sin ruta)");Array.isArray(t._exp)&&t._exp.length&&(o+='<div class="why">'+t._exp.slice(0,3).map(l=>`<span>${de(l.topic_key)}</span>`).join("")+"</div>"),en.querySelector(".tg").innerHTML=o;let a=`<i>${de(t.mem_type||"")}</i><i>${de(t.domain)}</i><i>centralidad ${t.heat}</i>`;Array.isArray(t._exp)&&(a+=t._exp.length?`<i style="color:var(--purple)">explicado \xD7${t._exp.length}</i>`:'<i style="opacity:.55">sin memorias</i>'),en.querySelector(".tm").innerHTML=a}else en.querySelector(".tg").textContent=t.gist||"(sin resumen)",en.querySelector(".tm").innerHTML=`<i>${de(t.domain)}</i><i>${de(t.mem_type||"sin tipo")}</i><i>calor ${t.heat}</i>`;let e=en.offsetWidth||220,i=en.offsetHeight||70,s=ia+16,r=sa+16;s+e>innerWidth-8&&(s=ia-e-16),r+i>innerHeight-8&&(r=sa-i-16),en.style.left=s+"px",en.style.top=r+"px",en.classList.add("on")}else en.classList.remove("on")}async function Zv(n){if(!(n._exp!==void 0||n._expLoading)){n._expLoading=!0;try{let t=await fetch("/api/explained?symbol="+encodeURIComponent(n.id),{cache:"no-store"});n._exp=t.ok?await t.json()||[]:[]}catch{n._exp=[]}finally{n._expLoading=!1}}}var Wo=2.4,Jo=new URLSearchParams(location.search).has("stats"),fr=null,Uc=0,Nc=0,Xo=0,ad=0;Jo&&(fr=document.createElement("div"),fr.style.cssText="position:fixed;left:50%;top:8px;transform:translateX(-50%);z-index:99;font:11px/1.5 ui-monospace,Menlo,monospace;color:#9fe8ff;background:rgba(6,10,18,.86);border:1px solid #23405c;border-radius:7px;padding:7px 12px;white-space:pre;pointer-events:none;letter-spacing:.02em;text-align:center",fr.textContent="midiendo\u2026",document.body.appendChild(fr));function wd(){if(requestAnimationFrame(wd),Me.info.reset(),si.activa()){si.frame(yn);return}let n=Jo?performance.now():0;(Os||se&&(ae!==Pt.length||(je?je.count:0)!==Ke.length))&&qv();let t=Pv(6);t&&Uv(Lc()>0?.16:1);let e=performance.now()/1e3;if(yn&&(ei*=.985,Nv()),se&&ae){ge>=0&&(Bs.setFromCamera(Ni,Mn),Bs.ray.intersectPlane(bd,ur)?Fs.worldToLocal(ur):ge=-1);let s=0,r=0,o=0;if(ge>=0){let a=ni[ge]+Math.sin(e*.5+ta[ge])*Wo,l=Di[ge]+Math.sin(e*.44+ea[ge])*Wo,h=Ui[ge]+Math.sin(e*.57+na[ge])*Wo;s=ur.x-a,r=ur.y-l,o=ur.z-h,As[ge]=s,Es[ge]=r,Ts[ge]=o}if(yn||ge>=0||Gc>.002||Wc||t){let a=0,l=!1,h=!1,f=se.instanceMatrix.array;for(let d=0;d<ae;d++){let c=Pt[d],u=yn?Wo:0,g=ni[d]+Math.sin(e*.5+ta[d])*u,y=Di[d]+Math.sin(e*.44+ea[d])*u,p=Ui[d]+Math.sin(e*.57+na[d])*u;if(d!==ge)if(Fi[d]>0){let H=Fi[d];As[d]+=(s*H-As[d])*.14,Es[d]+=(r*H-Es[d])*.14,Ts[d]+=(o*H-Ts[d])*.14}else As[d]*=.945,Es[d]*=.945,Ts[d]*=.945;let m=As[d],w=Es[d],_=Ts[d],A=Math.abs(m)+Math.abs(w)+Math.abs(_);A>a&&(a=A);let I=g+m,C=y+w,E=p+_;c.x=I,c.y=C,c.z=E;let T=d*16;f[T+12]=I,f[T+13]=C,f[T+14]=E,c.act>.06?(l=!0,we.set(c.col),sd.set(kc[c.ak]||dd),we.lerp(sd,Math.min(.85,c.act*.8)),se.setColorAt(d,we),Zo[d]=1,h=!0):Zo[d]&&(se.setColorAt(d,we.set(c.col)),Zo[d]=0,h=!0)}if(se.instanceMatrix.needsUpdate=!0,h&&se.instanceColor&&(se.instanceColor.needsUpdate=!0),je){Ps.uniforms.uTime.value=e;let d=je.instanceMatrix.array,c=Math.abs(ei-Xc)>.002;c&&(Xc=ei);let u=!1;for(let g=0;g<Ke.length;g++){let y=Ke[g],p=Pt[y.a],m=Pt[y.b],w=m.x-p.x,_=m.y-p.y,A=m.z-p.z,I=Math.sqrt(w*w+_*_+A*A)||1,C=1/I;w*=C,_*=C,A*=C;let E,T,H;Math.abs(_)<.9?(E=A,T=0,H=-w):(E=0,T=-A,H=_);let x=Math.sqrt(E*E+T*T+H*H)||1,S=1/x;E*=S,T*=S,H*=S;let O=T*A-H*_,W=H*w-E*A,q=E*_-T*w,et=y.__r,F=g*16;d[F]=E*et,d[F+1]=T*et,d[F+2]=H*et,d[F+3]=0,d[F+4]=w*I,d[F+5]=_*I,d[F+6]=A*I,d[F+7]=0,d[F+8]=O*et,d[F+9]=W*et,d[F+10]=q*et,d[F+11]=0,d[F+12]=(p.x+m.x)*.5,d[F+13]=(p.y+m.y)*.5,d[F+14]=(p.z+m.z)*.5,d[F+15]=1;let it=Math.max(p.act,m.act),Z=p.act>=m.act?p.ak:m.ak;it>.06&&Z>0?(we.set(kc[Z]),Ls[g]=.55+it*3.6,Is[g]=1+it*1.6,nn[g*3]=we.r,nn[g*3+1]=we.g,nn[g*3+2]=we.b,jo[g]=1,u=!0):(jo[g]||c)&&(we.set(fd(y)),Ls[g]=.5+ei*.35,Is[g]=.42+(y.confidence||0)*.5,nn[g*3]=we.r,nn[g*3+1]=we.g,nn[g*3+2]=we.b,jo[g]=0,u=!0)}if(je.instanceMatrix.needsUpdate=!0,u){let g=je.geometry.attributes;g.aColor.needsUpdate=!0,g.aSpeed.needsUpdate=!0,g.aGlow.needsUpdate=!0}}Gc=a,Wc=l}}let i=Jo?performance.now()-n:0;if(Bi.update(),Yv(),ks.render(),Jo){let s=performance.now();if(Uc+=s-n,Nc+=i,Xo++,s-ad>500){let r=Me.info.render,o=Me.info.memory,a=Uc/Xo,l=Nc/Xo;fr.textContent=Math.round(1e3/a)+" fps   frame "+a.toFixed(1)+" ms   \xB7   mis bucles "+l.toFixed(1)+" ms   \xB7   render+bloom "+(a-l).toFixed(1)+` ms
`+r.calls+" draw \xB7 "+(r.triangles/1e3).toFixed(0)+"k tris \xB7 dpr "+Me.getPixelRatio()+" \xB7 "+o.geometries+" geo \xB7 "+o.textures+" tex",Uc=Nc=Xo=0,ad=s}}}var Ht=n=>document.getElementById(n),de=n=>(n==null?"":String(n)).replace(/[&<>"]/g,t=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"})[t]);function qc(n){let t=_e==="code",e=n.brain||{neurons:[],synapses:[]},i=n.insights||{},s=i.observations||{},r=n.code||{nodes:[],edges:[]},o=t?(r.nodes||[]).length:(e.neurons||[]).length,a=t?r.total_nodes||0:e.total_neurons||0,l=t?r.truncated:e.truncated,h=t?(r.edges||[]).length:(e.synapses||[]).length,f=t?r.total_edges||0:e.total_synapses||0,c=(t?!!r.edges_truncated:!!e.synapses_truncated)?`${h}/${f}`:h;Ht("neuronCount").textContent=l?`${o}/${a}`:o,Ht("synCount").textContent=c,Ht("proj").textContent=n.project||"\u2014",Ht("ver").textContent=n.version||"";let u=(i.utilization||{}).active;Ht("kActive").textContent=t?a||o:u??(s.visible!=null?s.visible:s.active!=null?s.active:"\u2014"),Ht("kSyn").textContent=c;let g=(n.graph||{}).domains||[];Ht("kDomains").textContent=t?r.total_modules?r.truncated&&Bn.length<r.total_modules?`${Bn.length}/${r.total_modules}`:r.total_modules:Bn.length:g.length||Bn.length;let y=!t&&g.length?g.slice().sort((T,H)=>H.count-T.count||String(T.domain).localeCompare(String(H.domain))).slice(0,10).map(T=>({name:T.domain,count:T.count,color:Oi&&Oi.get(T.domain)||"#7f9cc9"})):Bn.slice(0,10);_e==="personas"?Kc():Ht("domlegend").innerHTML=y.length?y.map(T=>`<div class="lg"><span class="sw" style="background:${T.color};color:${T.color}"></span>${de(T.name)} <b>${T.count}</b></div>`).join(""):'<div class="empty">sin dominios</div>',_e==="personas"&&Ze&&(Ht("kActive").textContent=Ze.terminales.length,Ht("kSyn").textContent=Ze.despachos.reduce((T,H)=>T+H.veces,0),Ht("kDomains").textContent=xr.filter(T=>T.persona!=="(servicios)").length);let p=(n.orchestration||{}).runs||[];Ht("kRuns").textContent=p.filter(T=>T.status==="running"||T.done<T.total).length;let m=n.health||{},w=(m.checks||[]).filter(T=>T.status&&T.status!=="ok").length;Ht("health").className="pill "+(m.status==="ok"?"ok":"warn"),Ht("healthTxt").textContent=m.status==="ok"?"sano":w?`${w} avisos`:"revisar",Ht("checks").innerHTML=(m.checks||[]).length?(m.checks||[]).map(T=>{let H=!T.status||T.status==="ok";return`<div class="chk"><span class="s" style="background:${H?"var(--green)":"var(--amber)"};box-shadow:0 0 7px ${H?"var(--green)":"var(--amber)"}"></span>${de(T.code||T.name||"check")}</div>`}).join(""):'<div class="empty">sin checks</div>';let _=n.tokens||{},A=!!_.session_id,I=A&&_.status&&_.status!=="unbudgeted"&&_.budget;Ht("tokLabel").textContent=I?"Tokens de sesi\xF3n":A?"Tokens (sin techo)":"Tokens acumulados",Ht("tokVal").textContent=I?`${_.total||0} / ${_.budget}`:_.total||0;let C=Ht("tokBar");C.className="bar"+(I&&(_.status==="watch"||_.status==="over")?" watch":""),C.querySelector("i").style.width=Math.min(100,I?_.pct_used||0:_.total?8:0)+"%",Ht("runs").innerHTML=p.length?p.slice(0,6).map(T=>{let H=T.done||0,x=T.total||0,S=x?Math.round(H*100/x):0,O=T.status==="done"?"var(--green)":T.status==="failed"?"var(--red)":"var(--amber)";return`<div class="run"><span class="s" style="width:6px;height:6px;border-radius:50%;background:${O};box-shadow:0 0 7px ${O}"></span><span class="rl">${de(T.workflow_id||T.run_id)}</span><span class="rp">${H}/${x}\xB7${S}%</span></div>`}).join(""):'<div class="empty">sin runs activos</div>';let E=n.recent||[];Ht("recent").innerHTML=E.length?E.slice(0,6).map(T=>`<div class="item"><span class="tk">${de(T.topic_key)}</span><span class="gg">${de(T.gist)}</span></div>`).join(""):'<div class="empty">memoria en formaci\xF3n</div>'}var ye={nuevos:[],vistos:new Set,sondeos:[],enlace:{estado:"conectando"}},Ad=40,jv=600;function Kv(n){let t=(n.origen||"central")+"|"+(n.seq||0)+"|"+(n.at||"");return ye.vistos.has(t)?!0:(ye.vistos.add(t),ye.vistos.size>jv&&(ye.vistos=new Set([...ye.vistos].slice(-Ad*2))),!1)}var ld=0;function Jv(){let n=performance.now();n-ld<2e3||(ld=n,setTimeout(()=>{jc().catch(()=>{})},350))}function cd(n){if(!(!n||Kv(n))){if(n.kind==="sondeo"){ye.sondeos.push(Date.now());return}n.perdidos>0&&ye.nuevos.push({hueco:n.perdidos}),ye.nuevos.push(n),n.tool==="musubi_codegraph_push"&&(Ae.code=null),Jv()}}function Ed(n){let t=Date.parse(n);if(!isFinite(t))return"";let e=Math.max(0,Math.round((Date.now()-t)/1e3));if(e<60)return e+"s";let i=Math.round(e/60);return i<60?i+"m":Math.round(i/60)+"h"}var Qv=n=>String(n||"").replace(/^musubi_/,"");function $v(n){let t=document.createElement("div");if(n.hueco)return t.className="ev hueco",t.innerHTML=`<span class="s"></span><span class="t">se perdieron ${n.hueco} eventos</span>`,t;let e=n.origen==="local";return t.className="ev"+(e?" loc":"")+(n.outcome&&n.outcome!=="ok"?" err":""),t.dataset.at=n.at||"",t.innerHTML=`<span class="s"></span><span class="t">${de(Qv(n.tool))}</span>`+(e?'<span class="q aqui">esta maquina</span>':n.principal?`<span class="q">${de(n.principal)}</span>`:"")+`<span class="d">${Ed(n.at)}</span>`,t}function qo(){let n=Ht("vivo");if(n){if(ye.nuevos.length){let t=n.querySelector(".empty");t&&t.remove();for(let e of ye.nuevos)n.insertBefore($v(e),n.firstChild);for(ye.nuevos.length=0;n.children.length>Ad;)n.removeChild(n.lastChild)}n.children.length||(n.innerHTML='<div class="empty">sin trabajo todavia</div>'),t_()}}function t_(){let n=Ht("enlace");if(!n)return;let t=ye.enlace,e="enl"+(t.estado==="conectado"?" ok":t.estado==="conectando"?"":" mal"),i=t.estado==="conectado"?t.destino||"enlazado":t.estado==="conectando"?"conectando\u2026":t.estado==="apagado"?"apagado":"sin enlace";n.className!==e&&(n.className=e),n.textContent!==i&&(n.textContent=i);let s=t.detalle||"";n.title!==s&&(n.title=s)}function e_(){let n=Ht("vivo");if(n)for(let s of n.children){let r=s.dataset&&s.dataset.at;if(!r)continue;let o=s.lastElementChild;if(!o||!o.classList.contains("d"))continue;let a=Ed(r);o.textContent!==a&&(o.textContent=a)}let t=Date.now()-6e4;ye.sondeos=ye.sondeos.filter(s=>s>t);let e=Ht("latido"),i=Ht("latidoTxt");if(e&&i){let s=ye.sondeos.length,r=s>0?`sondeo \xB7 ${s}/min`:"sin sondeo";e.classList.toggle("vive",s>0),i.textContent!==r&&(i.textContent=r)}}setInterval(e_,1e3);function n_(n){if(_e!=="personas"||!si.activa()||!n||!n.tool)return;let t=zu(n,_d),e=ku(n);si.pulsar(t?{terminal:t.terminal,exacta:t.exacta,capa:e.capa,falla:e.falla,ms:e.ms}:{terminal:"",capa:e.capa,falla:e.falla,ms:e.ms})}function i_(){let n;try{n=new EventSource("/api/stream")}catch{return}n.addEventListener("backlog",t=>{try{(JSON.parse(t.data)||[]).forEach(cd)}catch{}qo()}),n.addEventListener("uso",t=>{try{let e=JSON.parse(t.data);cd(e),n_(e)}catch{}qo()}),n.addEventListener("enlace",t=>{try{ye.enlace=JSON.parse(t.data),qo()}catch{}}),n.onerror=()=>{ye.enlace.estado!=="apagado"&&(ye.enlace={estado:"caido",detalle:"se corto el stream del panel"},qo())}}function Td(){let n=innerWidth,t=innerHeight;Me.setSize(n,t),Mn.aspect=n/t,Mn.updateProjectionMatrix(),Me.getDrawingBufferSize(sn),ks.setSize(sn.x,sn.y),Bi.handleResize()}addEventListener("resize",Td);Td();var Ae={memory:null,code:null},Ds=null,Oc=null,dr={};async function ra(n){return dr[n]||(dr[n]=(async()=>{try{let t=await fetch("/api/graph?lens="+n,{cache:"no-store"});if(!t.ok)throw 0;Ae[n]=await t.json()}finally{dr[n]=null}})()),dr[n]}async function s_(){return ws||(ws=(async()=>{try{let n=await fetch("/api/actores",{cache:"no-store"});if(!n.ok)throw 0;let t=await n.json();pn={estado:t.estado,detalle:t.detalle||"",destino:t.destino||"",censo:t.censo||null}}catch{pn={estado:"caido",detalle:"el panel no pudo pedir el censo",censo:null}}finally{ws=null}})(),ws)}function r_(n,t){if(!n)return!1;if(t.touched&&t.touched.length){let e=new Map(n.neurons.map((i,s)=>[i.id,s]));for(let i of t.touched){let s=e.get(i.id);s!=null&&(n.neurons[s].heat=i.heat,n.neurons[s].recency_days=i.recency_days)}}if(t.new_neurons&&t.new_neurons.length){let e=new Set(n.neurons.map(i=>i.id));for(let i of t.new_neurons)e.has(i.id)||(n.neurons.push(i),e.add(i.id))}if(t.new_synapses&&t.new_synapses.length){let e=new Set(n.synapses.map(i=>i.source+"|"+i.target));for(let i of t.new_synapses){let s=i.source+"|"+i.target;e.has(s)||(n.synapses.push(i),e.add(s))}}return n.total_neurons=t.counts.neurons,n.total_synapses=t.counts.synapses,n.truncated=n.neurons.length<n.total_neurons,n.synapses_truncated=n.synapses.length<n.total_synapses,n.neurons.length===t.counts.neurons}function Yc(){let n=Ds||{};return{brain:Ae.memory||{neurons:[],synapses:[]},code:Ae.code||{nodes:[],edges:[]},insights:n.insights||{},graph:{domains:n.domains||[],total_observations:(n.counts||{}).neurons||0},health:n.health||{},tokens:n.tokens||{},recent:n.recent||[],orchestration:n.orchestration||{},project:n.project,version:n.version}}var Fc=!1;async function jc(){if(!Fc){Fc=!0;try{let n=await fetch("/api/pulse"+(Oc?"?since="+encodeURIComponent(Oc):""),{cache:"no-store"});if(!n.ok)throw 0;Ds=await n.json(),_e==="code"?Ae.code||await ra("code"):(!Ae.memory||!r_(Ae.memory,Ds))&&await ra("memory"),Oc=Ds.now,mr(),qc(Yc()),Ht("liveTxt").textContent="en vivo"}catch{Ht("liveTxt").textContent="reconectando"}finally{Fc=!1}}}var Bc=null;function mr(){if(_e==="personas"){if(!Ae.memory)return;let n=Hu(Ae.memory),t=Fu(n.terminales,pn&&pn.censo);n.actores=t.actores,n.sinDeclarar=t.sinDeclarar,n.censo=pn,xr=Vu(n.terminales,t.actores),_d=Bu(n.terminales,t.actores),Ze=n,si.cargar(n,xr),Kc();return}if(_e==="code"){if(!Ae.code||Bc===Ae.code)return;Dv(Ae.code),Bc=Ae.code;return}Ae.memory&&(Lv(Ae.memory),Bc=Ae.memory)}function Cd(n){yn=n;let t=Ht("motionBtn");t&&(t.textContent=yn?"\u275A\u275A pausar":"\u25B6 reanudar",t.classList.toggle("paused",!yn),t.setAttribute("aria-pressed",String(!yn)))}Ht("motionBtn").addEventListener("click",()=>Cd(!yn));Cd(yn);function Rd(n){_e=n;let t=Ht("lensBtn"),e=document.getElementById("brain"),i=document.getElementById("personas"),s=_e==="personas";if(e&&(e.hidden=s),i&&(i.hidden=!s),si.activar(s),t&&(t.textContent=s?"\u25C9 personas":_e==="code"?"\u25C9 c\xF3digo":"\u25C9 memoria",t.classList.toggle("code",_e==="code"),t.classList.toggle("personas",s),t.setAttribute("aria-pressed",String(_e!=="memory"))),o_(),_e==="code"&&!Ae.code){ra("code").then(()=>{mr(),Ds&&qc(Yc())}).catch(()=>{});return}s&&!pn&&!ws&&s_().then(()=>{mr()}).catch(()=>{}),mr(),Ds&&qc(Yc())}function o_(){let n=_e==="code",t=_e==="personas",e=(r,o)=>{let a=Ht(r);a&&(a.textContent=o)};if(e("lblNodes",n?"nodos":"neuronas"),e("lblEdges",n?"aristas":"sinapsis"),e("domTitle",t?"Personas":n?"M\xF3dulos":"Dominios"),e("lblActive",t?"Terminales":n?"Nodos":"Memorias activas"),e("lblSyn",t?"Despachos":n?"Aristas":"Sinapsis"),e("lblDomains",t?"Personas":n?"M\xF3dulos":"Dominios"),t){let r=Ht("actlegend");r&&(r.innerHTML=`<div class="lg"><span class="sw" style="background:rgba(255,255,255,.30);color:rgba(255,255,255,.30)"></span>sondeo</div><div class="lg"><span class="sw" style="background:#fff;color:#fff"></span>trabajo real</div><div class="lg"><span class="sw" style="background:#f5c451;color:#f5c451"></span>fall\xF3</div><div class="lg"><span class="sw" style="background:${wc};color:${wc}"></span>despacho</div><div class="lg"><span class="sw" style="background:${Ac};color:${Ac}"></span>cruza personas</div>`);let o=Ht("howto");o&&(o.innerHTML="<span><b>\xB7</b> cada neurona es una <b>terminal</b>; sus <b>dendritas</b>, cu\xE1nto escribi\xF3</span><span><b>\xB7</b> el <b>impulso</b> que recorre un \xE1rbol es <b>UNA llamada real a una tool</b>, en el momento en que ocurre. <b>Sin evento no hay luz</b>: si el cerebro est\xE1 quieto, esto est\xE1 quieto</span><span><b>\xB7</b> el <b>grosor</b> del impulso es lo que tard\xF3 esa llamada; el <b>\xE1mbar</b>, que fall\xF3</span><span><b>\xB7</b> la neurona <b>late</b> seg\xFAn cu\xE1nto se <b>recupera</b> lo que escribi\xF3 \u2014 la que nadie consulta se queda quieta</span><span><b>\xB7</b> el <b>color</b> agrupa por <b>persona</b> \xB7 la persona sale de qui\xE9n <b>firma</b>, no de qui\xE9n menciona</span>"),hd('<span><b>arrastr\xE1</b> gira</span><span class="sep">\xB7</span><span><b>rueda</b> acerca 40\xD7</span><span class="sep">\xB7</span><span><b>shift+arrastr\xE1</b> desplaza</span><span class="sep">\xB7</span><span><b>hover</b> detalla</span><span class="sep">\xB7</span><span><b>doble click</b> entra</span>'),Kc();return}hd('<span><b>arrastr\xE1</b> para rotar</span><span class="sep">\xB7</span><span><b>rueda</b> para acercar</span><span class="sep">\xB7</span><span><b>hover</b> revela el detalle</span>');let i=Ht("actlegend");i&&(i.innerHTML=n?`<div class="lg"><span class="sw" style="background:${Li.CALLS};color:${Li.CALLS}"></span>llama</div><div class="lg"><span class="sw" style="background:${Li.IMPORTS};color:${Li.IMPORTS}"></span>importa</div><div class="lg"><span class="sw" style="background:${Li.CONTAINS};color:${Li.CONTAINS}"></span>contiene</div>`:'<div class="lg"><span class="sw" style="background:#7f9cc9;color:#7f9cc9"></span>reposo</div><div class="lg"><span class="sw" style="background:#43e08b;color:#43e08b"></span>escribir</div><div class="lg"><span class="sw" style="background:#31c9ff;color:#31c9ff"></span>recordar</div><div class="lg"><span class="sw" style="background:#f5c451;color:#f5c451"></span>relacionar</div>');let s=Ht("howto");s&&(s.innerHTML=n?"<span><b>\xB7</b> cada punto es un <b>s\xEDmbolo</b> (funci\xF3n, tipo, archivo)</span><span><b>\xB7</b> las l\xEDneas son <b>llamadas / imports</b>; el color agrupa por <b>m\xF3dulo</b></span><span><b>\xB7</b> el <b>tama\xF1o</b> = centralidad \xB7 <b>hover</b> muestra qu\xE9 memorias lo explican</span>":"<span><b>\xB7</b> cada punto es una <b>memoria</b></span><span><b>\xB7</b> las l\xEDneas, <b>relaciones</b>; la luz que viaja = <b>recuerdo activ\xE1ndose</b></span><span><b>\xB7</b> el <b>color</b> agrupa por dominio \xB7 el <b>brillo</b>, recencia \xB7 el <b>tama\xF1o</b>, importancia</span>")}si.onFoco((n,t,e,i)=>{let s=document.getElementById("tip");if(!s)return;if(!n){s.classList.remove("on");return}let r=t&&t.despachos||[],o=(p,m)=>p.length?p.sort((w,_)=>_.veces-w.veces).slice(0,4).map(w=>`${de((m==="sale"?w.a:w.de).toLowerCase())} <b>${w.veces}</b>`).join(" \xB7 "):"\u2014",a=r.filter(p=>p.de===n.id),l=r.filter(p=>p.a===n.id),h=p=>(p||0).toLocaleString("es");if(s.querySelector(".tt").textContent=n.id.toLowerCase(),n.tipo==="actor"){let p=n.llamadas||{};s.querySelector(".tg").innerHTML=n.exacta===!1?`credencial de <b>${de(n.persona)}</b> \xB7 por convenci\xF3n del nombre, sin declarar`:`servicio \xB7 <b>due\xF1o no declarado</b>${p.proyecto?` \xB7 proyecto ${de(p.proyecto)}`:""}`,s.querySelector(".tm").innerHTML=`<i>${h(p.calls)} llamadas</i><i>${h(p.trabajo)} trabajo \xB7 ${h(p.sondeo)} sondeo</i><i>${p.tools||0} tools distintas</i>`}else{s.querySelector(".tg").innerHTML=`de <b>${de(n.persona||"sin autor")}</b> \xB7 ${n.notas} notas la nombran \xB7 ${n.firmas} las firma`;let p=n.llamadas;s.querySelector(".tm").innerHTML=`<i>calor ${n.calor}</i>${p?`<i>${h(p.calls)} llamadas \xB7 ${h(p.trabajo)} trabajo</i>`:""}<i>escribe a: ${o(a,"sale")}</i><i>recibe de: ${o(l,"entra")}</i>`}let f=s.offsetWidth||240,d=s.offsetHeight||80,c=typeof e=="number"?e:ia,u=typeof i=="number"?i:sa,g=c+16,y=u+16;g+f>innerWidth-8&&(g=c-f-16),y+d>innerHeight-8&&(y=u-d-16),s.style.left=g+"px",s.style.top=y+"px",s.classList.add("on")});function hd(n){let t=Ht("pistas");t&&(t.innerHTML=n)}function Kc(){let n=Ht("domlegend");if(!n)return;if(!xr.length){n.innerHTML='<div class="empty">sin terminales firmadas</div>';return}let t=xr.map((a,l)=>{let h=Xu(l),f=a.persona==="(servicios)"?`<b>${a.llamadas}</b> llamadas`:`<b>${a.notas}</b>`;return`<div class="lg"><span class="sw" style="background:${h};color:${h}"></span>${de(a.persona)} ${f}</div>`});pn&&pn.estado&&pn.estado!=="vivo"&&t.push(`<div class="lg" style="opacity:.7">censo de actores ${de(pn.estado)} \xB7 ${de(pn.detalle||"")}</div>`);let e=Ze&&Ze.actores?Ze.actores:[];if(e.length){let a=e.reduce((l,h)=>l+h.calls,0);t.push(`<div class="lg" style="opacity:.55">${e.length} actores \u25EF \xB7 ${a.toLocaleString("es")} llamadas</div>`)}let i=Ze&&Ze.sinDeclarar||0;i&&t.push(`<div class="lg" style="opacity:.55">${i} sin due\xF1o declarado \xB7 van a servicios</div>`);let s=Ze?Ze.despachos.length:0;s&&t.push(`<div class="lg" style="opacity:.55">${s} pares se escriben</div>`);let r=Ze?Ze.sinAutor:0;r&&t.push(`<div class="lg" style="opacity:.55">${r} notas sin autor \xB7 no se reparten</div>`);let o=si.cuentaPulsos();o.sinNeurona&&t.push(`<div class="lg" style="opacity:.55">${o.sinNeurona} eventos sin neurona \xB7 falta declarar su due\xF1o</div>`),n.innerHTML=t.join("")}var ud=Ht("lensBtn");ud&&ud.addEventListener("click",()=>Rd(_e==="memory"?"code":_e==="code"?"personas":"memory"));var zc=new URLSearchParams(location.search).get("lens");(zc==="code"||zc==="personas")&&Rd(zc);ra(_e==="code"?"code":"memory").then(()=>{mr()}).catch(()=>{});jc();setInterval(jc,5e3);i_();requestAnimationFrame(wd);})();
